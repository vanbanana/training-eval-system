package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// MigrateChatSessions migrates legacy chat_sessions/chat_messages to
// agent_sessions/agent_messages. It is fully idempotent and non-destructive:
//
//   - Original chat_sessions/chat_messages data is never deleted.
//   - Only sessions with migrated_at IS NULL and is_deleted = 0 are processed.
//   - Safe to call multiple times — subsequent calls are no-ops.
//
// The migration uses a permanent _chat_session_mapping table to track
// which agent_session IDs correspond to which chat_session IDs, enabling
// a clean rollback via cmd/rollback_chat.
func (db *DB) MigrateChatSessions(ctx context.Context) error {
	// Step 1: Add migrated_at column if it doesn't exist.
	if err := db.addColumnIfNotExists(ctx, "chat_sessions", "migrated_at", "TEXT"); err != nil {
		return fmt.Errorf("store: add migrated_at column: %w", err)
	}

	// Step 2: Create permanent mapping table.
	_, err := db.Writer.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _chat_session_migration (
			chat_session_id INTEGER PRIMARY KEY,
			agent_session_id INTEGER NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("store: create mapping table: %w", err)
	}

	// Step 3: Count sessions that need migration.
	var pending int
	err = db.Writer.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM chat_sessions WHERE is_deleted = 0 AND migrated_at IS NULL",
	).Scan(&pending)
	if err != nil {
		return fmt.Errorf("store: count pending: %w", err)
	}
	if pending == 0 {
		return nil // nothing to migrate
	}

	slog.Info("chat migration: starting", "pending_sessions", pending)

	// Step 4: Run migration inside a transaction.
	tx, err := db.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback()

	// 4a: Migrate sessions.
	res, err := tx.ExecContext(ctx, `
		INSERT INTO agent_sessions (owner_id, owner_role, agent_role, title, context_json, created_at, last_active_at)
		SELECT
			cs.student_id, 'student', 'student', cs.title,
			CASE WHEN cs.evaluation_id IS NOT NULL
				THEN '{"_migrated_from_chat_id":' || cs.id || ',"evaluation_id":' || cs.evaluation_id || '}'
				ELSE '{"_migrated_from_chat_id":' || cs.id || '}'
			END,
			cs.created_at, cs.last_active_at
		FROM chat_sessions cs
		WHERE cs.is_deleted = 0 AND cs.migrated_at IS NULL`)
	if err != nil {
		return fmt.Errorf("store: migrate sessions: %w", err)
	}
	sessionsMigrated, _ := res.RowsAffected()

	// 4b: Build old→new session ID mapping by matching the context_json marker.
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO _chat_session_migration (chat_session_id, agent_session_id)
		SELECT cs.id, asent.id
		FROM chat_sessions cs
		JOIN agent_sessions asent
			ON asent.context_json LIKE '%"_migrated_from_chat_id":' || cs.id || '%'
		WHERE cs.migrated_at IS NULL`)
	if err != nil {
		return fmt.Errorf("store: build mapping: %w", err)
	}

	// 4c: Migrate messages using the mapping.
	res, err = tx.ExecContext(ctx, `
		INSERT INTO agent_messages (session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at)
		SELECT csm.agent_session_id, cm.role, cm.content, cm.tool_call_id, cm.tool_name,
		       cm.prompt_tokens, cm.completion_tokens, cm.created_at
		FROM chat_messages cm
		JOIN _chat_session_migration csm ON csm.chat_session_id = cm.session_id`)
	if err != nil {
		return fmt.Errorf("store: migrate messages: %w", err)
	}
	messagesMigrated, _ := res.RowsAffected()

	// 4d: Mark migrated sessions.
	_, err = tx.ExecContext(ctx,
		"UPDATE chat_sessions SET migrated_at = datetime('now') WHERE migrated_at IS NULL AND is_deleted = 0")
	if err != nil {
		return fmt.Errorf("store: mark migrated: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}

	slog.Info("chat migration: complete",
		"sessions", sessionsMigrated, "messages", messagesMigrated)
	return nil
}

// addColumnIfNotExists adds a column to a table if it doesn't already exist.
func (db *DB) addColumnIfNotExists(ctx context.Context, table, column, colType string) error {
	var exists int
	err := db.Writer.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", table, column),
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check column: %w", err)
	}
	if exists > 0 {
		return nil
	}
	_, err = db.Writer.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	if err != nil {
		return fmt.Errorf("alter table: %w", err)
	}
	return nil
}

// ChatMigrationStats returns counts for verifying migration status.
func (db *DB) ChatMigrationStats(ctx context.Context) (pending, migrated, mappingCount int, err error) {
	err = db.Reader.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM chat_sessions WHERE is_deleted = 0 AND migrated_at IS NULL",
	).Scan(&pending)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, 0, nil // table doesn't exist or no rows
		}
		return 0, 0, 0, err
	}
	err = db.Reader.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM chat_sessions WHERE migrated_at IS NOT NULL",
	).Scan(&migrated)
	if err != nil {
		return pending, 0, 0, err
	}
	_ = db.Reader.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM _chat_session_migration",
	).Scan(&mappingCount)
	return pending, migrated, mappingCount, nil
}
