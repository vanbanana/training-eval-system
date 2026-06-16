package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/store"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	dbPath := os.Getenv("TES_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/app.db"
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Create seed users
	users := []struct {
		username    string
		displayName string
		password    string
		role        string
	}{
		{"admin", "系统管理员", "admin123", "admin"},
		{"teacher1", "张老师", "teacher123", "teacher"},
		{"student1", "李同学", "student123", "student"},
	}

	for _, u := range users {
		hash, _ := crypto.HashPassword(u.password)
		_, err := db.Writer.Exec(
			`INSERT OR IGNORE INTO users (username, display_name, password_hash, role, is_active)
			 VALUES (?, ?, ?, ?, 1)`,
			u.username, u.displayName, hash, u.role)
		if err != nil {
			log.Printf("failed to insert %s: %v", u.username, err)
		} else {
			fmt.Printf("✓ User: %s / %s (role: %s)\n", u.username, u.password, u.role)
		}
	}

	// Create a sample course
	_, _ = db.Writer.Exec(`INSERT OR IGNORE INTO courses (name, code) VALUES ('软件工程实训', 'SE101')`)
	fmt.Println("✓ Course: 软件工程实训 (SE101)")

	// Seed notifications for teacher1
	var teacherID int64
	err = db.Reader.QueryRow(`SELECT id FROM users WHERE username = 'teacher1'`).Scan(&teacherID)
	if err == nil {
		// Clear existing notifications first to keep the seed idempotent
		_, _ = db.Writer.Exec(`DELETE FROM notifications WHERE user_id = ?`, teacherID)

		notifications := []struct {
			Type      string
			Title     string
			Content   string
			Link      string
			CreatedAt string
		}{
			{
				Type:      "system.announcement",
				Title:     "智能实训评价管理系统正式上线",
				Content:   "欢迎使用智能实训评价管理系统！本系统集成了AI智能辅助评分、查重检测、多维度评价体系等核心功能。如有使用疑问，请查看系统使用手册或联系管理员。",
				Link:      "/notifications",
				CreatedAt: "2026-06-09 09:00:00",
			},
			{
				Type:      "evaluation.scored",
				Title:     "您有新的实训报告待批改",
				Content:   "【软件工程实训】有新的学生提交了报告。系统已自动完成AI辅助评分与查重分析，请尽快前往批改工作台完成人工复核与确认。",
				Link:      "/teacher/tasks",
				CreatedAt: "2026-06-10 12:30:00",
			},
			{
				Type:      "system.announcement",
				Title:     "AI智能评阅大模型参数优化公告",
				Content:   "系统已对底层AI评阅大模型完成参数调优与提示词模板升级，提升了评语生成的专业度与针对性。您可以在创建任务的评分维度中选择启用AI评分。",
				Link:      "/admin/llm",
				CreatedAt: "2026-06-10 14:35:00",
			},
		}

		for _, n := range notifications {
			_, err = db.Writer.Exec(
				`INSERT INTO notifications (user_id, type, title, content, is_read, link, created_at)
				 VALUES (?, ?, ?, ?, 0, ?, ?)`,
				teacherID, n.Type, n.Title, n.Content, n.Link, n.CreatedAt)
			if err != nil {
				log.Printf("failed to insert notification %q: %v", n.Title, err)
			}
		}
		fmt.Println("✓ Seeded notifications for teacher1")
	}

	fmt.Println("\n🎉 Seed complete! You can now login with:")
	fmt.Println("   admin / admin123")
	fmt.Println("   teacher1 / teacher123")
	fmt.Println("   student1 / student123")
}
