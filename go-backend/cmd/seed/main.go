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

	fmt.Println("\n🎉 Seed complete! You can now login with:")
	fmt.Println("   admin / admin123")
	fmt.Println("   teacher1 / teacher123")
	fmt.Println("   student1 / student123")
}
