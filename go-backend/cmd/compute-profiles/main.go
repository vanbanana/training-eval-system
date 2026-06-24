package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	_ "modernc.org/sqlite"

	"github.com/smartedu/training-eval-system/internal/config"
	"github.com/smartedu/training-eval-system/internal/llm"
	"github.com/smartedu/training-eval-system/internal/repository"
	"github.com/smartedu/training-eval-system/internal/service"
	"github.com/smartedu/training-eval-system/internal/store"
	"github.com/smartedu/training-eval-system/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	evalRepo := repository.NewEvaluationRepo(db)
	profileRepo := repository.NewProfileRepo(db)
	taskRepo := repository.NewTaskRepo(db)
	pool := worker.NewPool(4, 100)

	pc := service.NewProfileComputer(evalRepo, profileRepo, taskRepo, pool)

	if cfg.LLMAPIKey != "" {
		llmClient := llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMEmbedModel)
		if cfg.LLMOCRModel != "" {
			llmClient.SetOCRModel(cfg.LLMOCRModel)
		} else if cfg.LLMModel == "mimo-v2.5-pro" {
			llmClient.SetOCRModel("mimo-v2.5")
		}
		pc.SetLLMClient(llmClient)
	}

	// Get all student IDs that have scored evaluations
	rows, err := db.Reader.QueryContext(context.Background(),
		`SELECT DISTINCT e.student_id FROM evaluations e WHERE e.status IN ('scored', 'confirmed')`)
	if err != nil {
		slog.Error("query", "error", err)
		os.Exit(1)
	}
	defer rows.Close()

	var studentIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			studentIDs = append(studentIDs, id)
		}
	}

	fmt.Printf("Found %d students with scored evaluations\n", len(studentIDs))

	// Compute profiles synchronously (not via pool, to ensure completion)
	success := 0
	for _, sid := range studentIDs {
		err := pc.ComputeProfile(context.Background(), sid)
		if err != nil {
			fmt.Printf("  Student %d: FAILED - %v\n", sid, err)
		} else {
			success++
			fmt.Printf("  Student %d: OK\n", sid)
		}
	}

	fmt.Printf("\nComputed %d/%d profiles successfully\n", success, len(studentIDs))

	// Verify
	var count int
	db.Reader.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM student_profiles").Scan(&count)
	fmt.Printf("Total profiles in DB: %d\n", count)
}
