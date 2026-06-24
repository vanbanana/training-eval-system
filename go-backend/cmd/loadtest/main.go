package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	baseURL     = "http://localhost:8000"
	numStudents = 200
	uploadBurst = 50 // concurrent uploads per burst
	dbPath      = "data/app.db"
)

type uploadResult struct {
	studentID  int
	uploadOK   bool
	uploadMs   int64
	httpStatus int
	errMsg     string
}

func main() {
	log.SetFlags(0)
	fmt.Printf("=== 负载测试: %d 学生并发上传 ===\n", numStudents)
	fmt.Printf("并发批次: %d 人/批\n", uploadBurst)
	fmt.Println()

	// Step 1: Login as teacher
	teacherToken := login("teacher1", "teacher123")
	if teacherToken == "" {
		log.Fatal("Teacher login failed")
	}
	fmt.Println("[1/7] Teacher登录成功")

	// Step 2: Use existing course and task (course 1, task 1)
	courseID := int64(1)
	taskID := int64(1)
	fmt.Printf("[2/7] 使用现有课程ID=%d, 任务ID=%d\n", courseID, taskID)

	// Step 3: Create 200 student accounts
	fmt.Printf("[3/7] 创建 %d 个学生账号...\n", numStudents)
	studentTokens := createStudents(numStudents)
	validTokens := 0
	for _, t := range studentTokens {
		if t != "" {
			validTokens++
		}
	}
	fmt.Printf("      成功获取 %d 个token\n", validTokens)

	// Step 4: Enroll students
	enrollStudents(teacherToken, courseID, numStudents)
	fmt.Printf("[4/7] 学生已加入课程\n")

	// Step 5: Find .doc file
	docFile := findDocFile()
	if docFile == "" {
		log.Fatal("找不到.doc文件")
	}
	fmt.Printf("[5/7] 上传文件: %s (%d KB)\n", filepath.Base(docFile), fileSize(docFile)/1024)

	// Step 6: Concurrent upload
	fmt.Printf("[6/7] 开始 %d 人并发上传...\n", numStudents)
	testStart := time.Now()

	var (
		successCount  int64
		failCount     int64
		totalUploadMs int64
		results       []uploadResult
		resultsMu     sync.Mutex
	)

	for burstStart := 0; burstStart < numStudents; burstStart += uploadBurst {
		burstEnd := burstStart + uploadBurst
		if burstEnd > numStudents {
			burstEnd = numStudents
		}

		var wg sync.WaitGroup
		for i := burstStart; i < burstEnd; i++ {
			if studentTokens[i] == "" {
				atomic.AddInt64(&failCount, 1)
				continue
			}
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				start := time.Now()
				ok, status, errMsg := uploadFile(studentTokens[idx], taskID, docFile)
				elapsed := time.Since(start).Milliseconds()

				r := uploadResult{idx + 1, ok, elapsed, status, errMsg}
				if ok {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}
				atomic.AddInt64(&totalUploadMs, elapsed)

				resultsMu.Lock()
				results = append(results, r)
				resultsMu.Unlock()
			}(i)
		}
		wg.Wait()

		if burstEnd < numStudents {
			time.Sleep(200 * time.Millisecond)
		}
	}

	uploadTime := time.Since(testStart)
	avgMs := float64(0)
	if successCount > 0 {
		avgMs = float64(totalUploadMs) / float64(successCount)
	}

	fmt.Println("\n=== 上传阶段结果 ===")
	fmt.Printf("上传总耗时: %.1fs\n", uploadTime.Seconds())
	fmt.Printf("成功: %d, 失败: %d\n", successCount, failCount)
	fmt.Printf("平均上传耗时: %.0fms\n", avgMs)
	fmt.Printf("上传吞吐: %.1f uploads/s\n", float64(successCount)/uploadTime.Seconds())

	// Show failure examples
	failShown := 0
	for _, r := range results {
		if !r.uploadOK && failShown < 3 {
			fmt.Printf("  失败: 学生%d HTTP%d %s\n", r.studentID, r.httpStatus, r.errMsg)
			failShown++
		}
	}

	// Step 7: Monitor pipeline
	fmt.Printf("\n[7/7] 等待流水线处理...\n")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	deadline := time.Now().Add(60 * time.Minute)
	lastParsed, lastScored := int64(0), int64(0)
	stallCount := 0

	for time.Now().Before(deadline) {
		time.Sleep(15 * time.Second)
		parsed, scored, parsing, pending := getStats(db)
		elapsed := time.Since(testStart)

		// Progress indicator
		pendingStr := ""
		if pending > 0 {
			pendingStr = fmt.Sprintf(", pending: %d", pending)
		}
		parsingStr := ""
		if parsing > 0 {
			parsingStr = fmt.Sprintf(", parsing: %d", parsing)
		}
		fmt.Printf("[%6.1fs] parsed: %d, scored: %d%s%s\n",
			elapsed.Seconds(), parsed, scored, parsingStr, pendingStr)

		// Check if stalled
		if parsed == lastParsed && scored == lastScored {
			stallCount++
		} else {
			stallCount = 0
		}
		lastParsed = parsed
		lastScored = scored

		if stallCount >= 8 && parsed+scored > 0 {
			fmt.Println("Pipeline appears stalled (no progress for 2 min)")
			break
		}

		if parsed >= successCount && scored >= successCount {
			fmt.Println("\n全部评分完成!")
			break
		}
	}

	// Final report
	finalParsed, finalScored, _, _ := getStats(db)
	totalTime := time.Since(testStart)

	// Score distribution
	var avgScore, minScore, maxScore float64
	db.QueryRow("SELECT COALESCE(AVG(total_score),0), COALESCE(MIN(total_score),0), COALESCE(MAX(total_score),0) FROM evaluations WHERE status='scored'").Scan(&avgScore, &minScore, &maxScore)

	fmt.Println("\n========== 最终报告 ==========")
	fmt.Printf("学生数: %d\n", numStudents)
	fmt.Printf("上传成功: %d, 失败: %d\n", successCount, failCount)
	fmt.Printf("解析完成: %d\n", finalParsed)
	fmt.Printf("评分完成: %d\n", finalScored)
	fmt.Printf("总耗时: %.1fmin\n", totalTime.Minutes())
	fmt.Printf("系统吞吐: %.1f eval/hour\n", float64(finalScored)/totalTime.Hours())
	fmt.Printf("评分分布: 平均=%.1f, 最低=%.1f, 最高=%.1f\n", avgScore, minScore, maxScore)
	fmt.Printf("成功率: %.1f%% (上传), %.1f%% (评分)\n",
		float64(successCount)/float64(numStudents)*100,
		float64(finalScored)/float64(numStudents)*100)
}

func login(username, password string) string {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if token, ok := result["access_token"].(string); ok {
		return token
	}
	return ""
}

func doRequest(method, url, token string, body []byte) (*http.Response, map[string]interface{}) {
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	resp.Body.Close()
	return resp, data
}

func ensureCourseAndTask(token string) (int64, int64) {
	// Use existing course/task (id=1 and id=1)
	// For load test, create new ones
	body, _ := json.Marshal(map[string]string{
		"name": "负载测试课程",
		"code": fmt.Sprintf("LOAD-%d", time.Now().Unix()),
	})
	resp, data := doRequest("POST", baseURL+"/api/courses", token, body)
	if resp == nil || resp.StatusCode != 200 && resp.StatusCode != 201 {
		// Fallback: use existing course 1
		return 1, 1
	}
	courseID := int64(data["id"].(float64))

	// Create task
	body, _ = json.Marshal(map[string]string{
		"title":       "负载测试-并发上传实验",
		"description": "测试系统在200人并发上传时的表现",
	})
	resp, data = doRequest("POST", fmt.Sprintf("%s/api/courses/%d/tasks", baseURL, courseID), token, body)
	if resp == nil || data == nil {
		return courseID, 1
	}
	taskID := int64(data["id"].(float64))

	// Add dimensions
	dims := []map[string]interface{}{
		{"name": "命令正确性", "weight": 30, "order_index": 1},
		{"name": "操作完整性", "weight": 30, "order_index": 2},
		{"name": "结果分析", "weight": 20, "order_index": 3},
		{"name": "格式规范", "weight": 20, "order_index": 4},
	}
	dimsBody, _ := json.Marshal(map[string]interface{}{"dimensions": dims})
	doRequest("PUT", fmt.Sprintf("%s/api/tasks/%d/dimensions", baseURL, taskID), token, dimsBody)

	// Publish
	doRequest("POST", fmt.Sprintf("%s/api/tasks/%d/publish", baseURL, taskID), token, nil)

	return courseID, taskID
}

func createStudents(n int) []string {
	tokens := make([]string, n)

	// First, get admin token to create users
	adminToken := login("admin", "admin123")
	if adminToken == "" {
		// Try with teacher
		adminToken = login("teacher1", "teacher123")
	}
	if adminToken == "" {
		fmt.Println("WARNING: Cannot get admin/teacher token for user creation")
		return tokens
	}

	// Create users via admin API (POST /api/users)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			username := fmt.Sprintf("loadstu%d", idx+1)
			body, _ := json.Marshal(map[string]string{
				"username":  username,
				"password":  "student123",
				"role":      "student",
				"real_name": fmt.Sprintf("负载学生%d", idx+1),
			})
			req, _ := http.NewRequest("POST", baseURL+"/api/users", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+adminToken)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			if err != nil {
				return
			}

			// Login to get token
			tokens[idx] = login(username, "student123")
		}(i)
	}
	wg.Wait()
	return tokens
}

func enrollStudents(token string, courseID int64, n int) {
	// Get all load test student IDs
	resp, data := doRequest("GET", fmt.Sprintf("%s/api/users?role=student&limit=500", baseURL), token, nil)
	if resp == nil || data == nil {
		return
	}

	var studentIDs []float64
	if items, ok := data["items"].([]interface{}); ok {
		for _, item := range items {
			if u, ok := item.(map[string]interface{}); ok {
				if u["role"].(string) == "student" {
					username := ""
					if un, ok := u["username"].(string); ok {
						username = un
					}
					if strings.HasPrefix(username, "loadstu") {
						studentIDs = append(studentIDs, u["id"].(float64))
					}
				}
			}
		}
	}

	// Enroll in batches of 50
	for i := 0; i < len(studentIDs); i += 50 {
		end := i + 50
		if end > len(studentIDs) {
			end = len(studentIDs)
		}
		batch := studentIDs[i:end]
		body, _ := json.Marshal(map[string]interface{}{"student_ids": batch})
		doRequest("POST", fmt.Sprintf("%s/api/courses/%d/enroll", baseURL, courseID), token, body)
	}
}

func findDocFile() string {
	entries, err := os.ReadDir("data/uploads")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := os.ReadDir(filepath.Join("data/uploads", e.Name()))
		if err != nil {
			continue
		}
		for _, s := range sub {
			if !s.IsDir() {
				continue
			}
			files, err := os.ReadDir(filepath.Join("data/uploads", e.Name(), s.Name()))
			if err != nil {
				continue
			}
			for _, f := range files {
				if strings.HasSuffix(strings.ToLower(f.Name()), ".doc") {
					return filepath.Join("data/uploads", e.Name(), s.Name(), f.Name())
				}
			}
		}
	}
	return ""
}

func fileSize(p string) int64 {
	info, _ := os.Stat(p)
	if info != nil {
		return info.Size()
	}
	return 0
}

func uploadFile(token string, taskID int64, filePath string) (bool, int, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return false, 0, err.Error()
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, 0, err.Error()
	}
	defer f.Close()
	io.Copy(part, f)
	writer.Close()

	// Upload API: POST /api/uploads/{taskId}
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/uploads/%d", baseURL, taskID), body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return true, resp.StatusCode, ""
	}
	respBody, _ := io.ReadAll(resp.Body)
	msg := string(respBody)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return false, resp.StatusCode, msg
}

func getStats(db *sql.DB) (parsed, scored, parsing, pending int64) {
	rows, err := db.Query("SELECT count(*), parse_status FROM uploads GROUP BY parse_status")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var count int64
		var status string
		rows.Scan(&count, &status)
		switch status {
		case "parsed":
			parsed = count
		case "parsing":
			parsing = count
		default:
			pending += count
		}
	}

	rows2, err := db.Query("SELECT count(*), status FROM evaluations GROUP BY status")
	if err != nil {
		return
	}
	defer rows2.Close()
	for rows2.Next() {
		var count int64
		var status string
		rows2.Scan(&count, &status)
		if status == "scored" {
			scored = count
		}
	}
	return
}
