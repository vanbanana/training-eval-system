// Command rollback_chat reverses the chat_sessions → agent_sessions migration.
//
// Usage:
//
//	go run ./cmd/rollback_chat [--db-path /path/to/app.db]
//
// The rollback:
//   - Deletes migrated agent_messages and agent_sessions (tracked by _chat_session_migration).
//   - Drops the _chat_session_migration mapping table.
//   - Clears migrated_at on chat_sessions, restoring them to the legacy code path.
//
// Original chat_sessions and chat_messages data are never modified by the
// forward migration, so rollback always preserves the original history.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	dbPath := flag.String("db-path", "", "path to SQLite database (default: TES_DB_PATH or ./data/app.db)")
	dryRun := flag.Bool("dry-run", false, "show what would be deleted without making changes")
	flag.Parse()

	if *dbPath == "" {
		*dbPath = os.Getenv("TES_DB_PATH")
	}
	if *dbPath == "" {
		*dbPath = "./data/app.db"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open("sqlite", *dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Check if mapping table exists.
	var hasMapping int
	_ = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='_chat_session_migration'",
	).Scan(&hasMapping)

	if hasMapping == 0 {
		fmt.Println("No migration mapping table found — nothing to roll back.")
		return
	}

	// Count rows to be affected.
	var msgCount, sessCount int
	_ = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM agent_messages WHERE session_id IN (SELECT agent_session_id FROM _chat_session_migration)",
	).Scan(&msgCount)
	_ = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM _chat_session_migration",
	).Scan(&sessCount)

	fmt.Printf("Rollback will affect:\n")
	fmt.Printf("  agent_messages: %d rows\n", msgCount)
	fmt.Printf("  agent_sessions: %d rows\n", sessCount)

	if *dryRun {
		fmt.Println("Dry run — no changes made.")
		return
	}

	// Delete migrated agent messages.
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM agent_messages WHERE session_id IN (SELECT agent_session_id FROM _chat_session_migration)",
	); err != nil {
		log.Fatalf("delete migrated messages: %v", err)
	}

	// Delete migrated agent sessions.
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM agent_sessions WHERE id IN (SELECT agent_session_id FROM _chat_session_migration)",
	); err != nil {
		log.Fatalf("delete migrated sessions: %v", err)
	}

	// Drop mapping table.
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS _chat_session_migration"); err != nil {
		log.Fatalf("drop mapping table: %v", err)
	}

	// Clear migrated_at on chat_sessions.
	res, err := tx.ExecContext(ctx, "UPDATE chat_sessions SET migrated_at = NULL WHERE migrated_at IS NOT NULL")
	if err != nil {
		log.Fatalf("clear migrated_at: %v", err)
	}
	restored, _ := res.RowsAffected()

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	fmt.Printf("Rollback complete:\n")
	fmt.Printf("  Deleted %d migrated agent messages\n", msgCount)
	fmt.Printf("  Deleted %d migrated agent sessions\n", sessCount)
	fmt.Printf("  Restored %d chat sessions to legacy path\n", restored)
	fmt.Println("Original chat history is intact and accessible.")
}
