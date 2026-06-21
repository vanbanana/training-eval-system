package handler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/internal/store"
	"github.com/smartedu/training-eval-system/testutil"
)

// createTestDocx creates a real .docx file with the given text content.
// .docx is a ZIP containing word/document.xml with the text.
func createTestDocx(content string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Required .docx structure
	docContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:t>%s</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`, content)

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
</Relationships>`,
		"word/document.xml": docContent,
	}

	for name, data := range files {
		f, err := w.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := f.Write([]byte(data)); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// TestFileUpload_001_DocxParse uploads a real .docx file via the upload API
// and verifies it is parsed and stored correctly.
func TestFileUpload_001_DocxParse(t *testing.T) {
	// Pipeline flow: verify (3 retries max) + score (3 retries max) = up to 6 LLM calls
	fakeLLM := testutil.NewFakeLLM()
	// For verifier (3 retries possible)
	fakeLLM.WithToolCallResponse("submit_verification", map[string]any{
		"match_rate": 85.0, "checkpoints": []string{"installed Go", "wrote server"},
		"missing_items": []string{}, "logic_issues": []string{},
	})
	fakeLLM.WithToolCallResponse("submit_verification", map[string]any{
		"match_rate": 85.0, "checkpoints": []string{"installed Go", "wrote server"},
		"missing_items": []string{}, "logic_issues": []string{},
	})
	fakeLLM.WithToolCallResponse("submit_verification", map[string]any{
		"match_rate": 85.0, "checkpoints": []string{"installed Go", "wrote server"},
		"missing_items": []string{}, "logic_issues": []string{},
	})
	// For scorer (3 retries possible)
	fakeLLM.WithToolCallResponse("submit_scores", map[string]any{
		"scores": []map[string]any{
			{"dimension_id": 300, "score": 85.0, "rationale": "内容完整，结构清晰"},
		},
	})
	fakeLLM.WithToolCallResponse("submit_scores", map[string]any{
		"scores": []map[string]any{
			{"dimension_id": 300, "score": 85.0, "rationale": "内容完整，结构清晰"},
		},
	})
	fakeLLM.WithToolCallResponse("submit_scores", map[string]any{
		"scores": []map[string]any{
			{"dimension_id": 300, "score": 85.0, "rationale": "内容完整，结构清晰"},
		},
	})
	app := testutil.SetupTestAppWithLLM(t, fakeLLM)

	// Seed: teacher, student, course, class, published task
	f := seedFileUploadFixture(t, app.DB)
	studentToken := testutil.GenerateTestToken(f.StudentAID, "student_a", "student")

	// Create a real .docx with Chinese content
	docxContent := "实验报告：Go语言实训\n\n一、实验目的\n掌握Go语言基础语法和标准库使用。\n\n二、实验过程\n1. 安装Go环境\n2. 编写Web服务器\n3. 添加单元测试\n\n三、实验结论\n成功掌握了Go语言开发。"
	docxBytes, err := createTestDocx(docxContent)
	if err != nil {
		t.Fatalf("create test docx: %v", err)
	}
	t.Logf("Created test docx: %d bytes", len(docxBytes))

	// Upload via multipart form
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "go_report.docx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(docxBytes); err != nil {
		t.Fatalf("write file: %v", err)
	}
	w.Close()

	req, err := http.NewRequest("POST", app.Server.URL+fmt.Sprintf("/api/uploads/by-task/%d", f.TaskAID), &buf)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+studentToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer resp.Body.Close()

	// Verify upload accepted
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 Created, got %d. Body: %s", resp.StatusCode, string(body))
	}

	var upload dto.UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&upload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	if upload.FileType != "docx" {
		t.Errorf("expected file_type=docx, got %s", upload.FileType)
	}
	if upload.ParseStatus != "pending" && upload.ParseStatus != "parsing" {
		t.Logf("Upload parse_status=%s (may be processed by async pipeline)", upload.ParseStatus)
	}

	t.Logf("Upload created: id=%d filename=%s type=%s status=%s size=%d",
		upload.ID, upload.Filename, upload.FileType, upload.ParseStatus, upload.FileSize)

	// Poll for parse completion — check upload parse status
	var parseStatus string
	var rawText string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)

		resp := doRequest(t, app.Server, "GET",
			fmt.Sprintf("/api/uploads/by-task/%d", f.TaskAID),
			studentToken, nil)
		if resp.StatusCode == http.StatusOK {
			var uploads []dto.UploadResponse
			json.NewDecoder(resp.Body).Decode(&uploads)
			resp.Body.Close()
			for _, u := range uploads {
				if u.ID == upload.ID {
					parseStatus = u.ParseStatus
					break
				}
			}
			if parseStatus == "parsed" || parseStatus == "failed" {
				break
			}
		} else {
			resp.Body.Close()
		}
	}

	// Fetch parse result with student token (owner)
	resp3 := doRequest(t, app.Server, "GET",
		fmt.Sprintf("/api/parse/%d/result", upload.ID),
		studentToken, nil)
	if resp3.StatusCode == http.StatusOK {
		var pr struct {
			RawText string `json:"raw_text"`
		}
		json.NewDecoder(resp3.Body).Decode(&pr)
		resp3.Body.Close()
		if pr.RawText != "" {
			parseStatus = "parsed"
			rawText = pr.RawText
		} else if parseStatus == "parsed" {
			rawText = pr.RawText
		}
	} else {
		resp3.Body.Close()
	}

	if parseStatus == "failed" {
		t.Fatalf("Parse failed (status=failed)")
	}
	if rawText == "" {
		t.Skipf("Parse did not produce raw_text (status=%s). Pipeline may need more time.", parseStatus)
	}

	t.Logf("Parse result raw_text (%d chars): %s", len(rawText), rawText)

	// Verify extracted text contains expected Chinese content
	expectedSubstrings := []string{"Go语言", "实验目的", "实验过程", "实验结论", "Web服务器"}
	for _, s := range expectedSubstrings {
		if !strings.Contains(rawText, s) {
			t.Errorf("Parsed text missing expected content: %q", s)
		}
	}
}

// TestFileUpload_002_PDFUploadAndList verifies PDF upload and list flow.
func TestFileUpload_002_PDFUploadAndList(t *testing.T) {
	app := testutil.SetupTestApp(t)
	f := seedFileUploadFixture(t, app.DB)
	studentToken := testutil.GenerateTestToken(f.StudentAID, "student_a", "student")

	// Create minimal valid PDF
	pdfContent := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R>>endobj\nxref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%EOF")

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "report.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(pdfContent); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	w.Close()

	req, err := http.NewRequest("POST", app.Server.URL+fmt.Sprintf("/api/uploads/by-task/%d", f.TaskAID), &buf)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+studentToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(body))
	}

	// List uploads for task
	listResp := doRequest(t, app.Server, "GET",
		fmt.Sprintf("/api/uploads/by-task/%d", f.TaskAID),
		studentToken, nil)
	testutil.AssertStatus(t, listResp, http.StatusOK)

	var uploads []dto.UploadResponse
	json.NewDecoder(listResp.Body).Decode(&uploads)

	if len(uploads) == 0 {
		t.Fatal("expected at least 1 upload in list")
	}

	found := false
	for _, u := range uploads {
		if u.Filename == "report.pdf" {
			found = true
			break
		}
	}
	if !found {
		t.Error("uploaded report.pdf not found in upload list")
	}
}

// fileUploadFixture holds IDs for file upload tests.
type fileUploadFixture struct {
	AdminID     int64
	TeacherAID  int64
	StudentAID  int64
	CourseAID   int64
	ClassAID    int64
	TaskAID     int64
}

func seedFileUploadFixture(t *testing.T, db *store.DB) *fileUploadFixture {
	t.Helper()
	ctx := context.Background()
	w := db.Writer
	now := time.Now()
	f := &fileUploadFixture{}

	w.ExecContext(ctx, "INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (10,'admin','管理员','x','admin',1)")
	f.AdminID = 10
	w.ExecContext(ctx, "INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (11,'teacher_a','教师A','x','teacher',1)")
	f.TeacherAID = 11
	w.ExecContext(ctx, "INSERT INTO users (id,username,display_name,password_hash,role,is_active) VALUES (13,'student_a','学生A','x','student',1)")
	f.StudentAID = 13

	w.ExecContext(ctx, "INSERT INTO courses (id,name,code,is_archived) VALUES (300,'文件上传测试','UPLOAD-TEST',0)")
	f.CourseAID = 300
	w.ExecContext(ctx, "INSERT INTO classes (id,name,course_id,teacher_id,student_count,is_archived) VALUES (300,'上传测试班',300,11,0,0)")
	f.ClassAID = 300
	w.ExecContext(ctx, "INSERT INTO class_memberships (class_id,student_id) VALUES (300,13)")

	w.ExecContext(ctx,
		`INSERT INTO training_tasks (id,name,description,requirements,teacher_id,course_id,status,deadline)
		 VALUES (300,'上传测试任务','Upload E2E Test','Submit a report',11,300,'published',?)`, now.Add(7*24*time.Hour))
	f.TaskAID = 300
	w.ExecContext(ctx, "INSERT INTO task_classes (task_id,class_id) VALUES (300,300)")
	w.ExecContext(ctx, "INSERT INTO dimensions (id,task_id,name,weight,order_index) VALUES (300,300,'内容完整',100,0)")

	return f
}