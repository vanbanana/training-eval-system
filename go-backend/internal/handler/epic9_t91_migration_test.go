package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/dto"
	"github.com/smartedu/training-eval-system/testutil"
)

// applyChatMigration runs the chat_sessions → agent_sessions migration via the store API.
func applyChatMigration(t *testing.T, app *testutil.TestApp) {
	t.Helper()
	if err := app.DB.MigrateChatSessions(context.Background()); err != nil {
		t.Fatalf("apply chat migration: %v", err)
	}
}

// seedChatData inserts legacy chat sessions and messages for testing.
func seedChatData(t *testing.T, app *testutil.TestApp, studentID int64, sessionCount, msgsPerSession int) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	for i := 0; i < sessionCount; i++ {
		sessID := 300 + int(studentID)*100 + i
		_, err := app.DB.Writer.Exec(
			`INSERT INTO chat_sessions (id, student_id, title, created_at, last_active_at)
			 VALUES (?, ?, ?, ?, ?)`,
			sessID, studentID,
			fmt.Sprintf("Chat Session %d", i),
			now, now,
		)
		if err != nil {
			t.Fatalf("seed chat session: %v", err)
		}
		for j := 0; j < msgsPerSession; j++ {
			role := "user"
			if j%2 == 1 {
				role = "assistant"
			}
			_, err := app.DB.Writer.Exec(
				`INSERT INTO chat_messages (session_id, role, content, created_at)
				 VALUES (?, ?, ?, ?)`,
				sessID, role, fmt.Sprintf("message %d", j), now,
			)
			if err != nil {
				t.Fatalf("seed chat message: %v", err)
			}
		}
	}
}

// countRows returns the row count for a given table.
func countRows(t *testing.T, app *testutil.TestApp, table string) int {
	t.Helper()
	var count int
	err := app.DB.Reader.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	if err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// TEST-T9.1-01: Empty DB migration succeeds.
func TestT91_01_EmptyDBMigration(t *testing.T) {
	app := testutil.SetupTestApp(t)
	// No chat_sessions or chat_messages exist — migration must succeed.
	applyChatMigration(t, app)

	// Verify migrated_at column exists and mapping table was created.
	var colCount int
	err := app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('chat_sessions') WHERE name='migrated_at'",
	).Scan(&colCount)
	if err != nil {
		t.Fatalf("check migrated_at column: %v", err)
	}
	if colCount != 1 {
		t.Fatal("migrated_at column not found on chat_sessions")
	}

	var tableCount int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='_chat_session_migration'",
	).Scan(&tableCount)
	if err != nil {
		t.Fatalf("check mapping table: %v", err)
	}
	if tableCount != 1 {
		t.Fatal("_chat_session_migration table not found")
	}
}

// TEST-T9.1-02: Legacy student chat sessions and messages are migrated.
func TestT91_02_StudentSessionMigration(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	// Seed legacy chat data for studentA (user_id=13).
	seedChatData(t, app, 13, 3, 5) // 3 sessions, 5 messages each

	// Record counts before migration. The fixture also seeds 2 chat_sessions (id=200, 201)
	// with no messages, so the migration will process those too.
	beforeSessions := countRows(t, app, "agent_sessions")
	beforeMessages := countRows(t, app, "agent_messages")
	beforeChatSessions := countRows(t, app, "chat_sessions")

	applyChatMigration(t, app)

	// Verify agent_sessions gained exactly as many rows as there are un-migrated chat sessions.
	afterSessions := countRows(t, app, "agent_sessions")
	sessionDelta := afterSessions - beforeSessions
	if sessionDelta != beforeChatSessions {
		t.Errorf("expected %d migrated sessions (= chat_sessions count), got %d (before=%d, after=%d)",
			beforeChatSessions, sessionDelta, beforeSessions, afterSessions)
	}

	// Verify agent_messages gained 15 new rows (3 seeded sessions × 5 messages each).
	// The fixture's chat_sessions have no messages.
	afterMessages := countRows(t, app, "agent_messages")
	msgDelta := afterMessages - beforeMessages
	if msgDelta != 15 {
		t.Errorf("expected 15 migrated messages, got %d (before=%d, after=%d)",
			msgDelta, beforeMessages, afterMessages)
	}

	// Verify migrated sessions are visible through the API.
	resp := doRequest(t, app.Server, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
	var sessions []dto.AgentSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	resp.Body.Close()

	// Seeded sessions should appear in the API response.
	migratedCount := 0
	for _, s := range sessions {
		if s.Title == "Chat Session 0" || s.Title == "Chat Session 1" || s.Title == "Chat Session 2" {
			migratedCount++
		}
	}
	if migratedCount != 3 {
		t.Errorf("expected 3 seeded migrated sessions in API response, found %d (total sessions: %d)",
			migratedCount, len(sessions))
	}
}

// TEST-T9.1-03: Migration is idempotent — running twice doesn't duplicate data.
func TestT91_03_IdempotentMigration(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	seedChatData(t, app, 13, 2, 3)

	// First migration.
	applyChatMigration(t, app)
	firstSessions := countRows(t, app, "agent_sessions")
	firstMessages := countRows(t, app, "agent_messages")

	// Second migration — should be a no-op.
	applyChatMigration(t, app)
	secondSessions := countRows(t, app, "agent_sessions")
	secondMessages := countRows(t, app, "agent_messages")

	if firstSessions != secondSessions {
		t.Errorf("sessions duplicated: first=%d, second=%d", firstSessions, secondSessions)
	}
	if firstMessages != secondMessages {
		t.Errorf("messages duplicated: first=%d, second=%d", firstMessages, secondMessages)
	}
}

// TEST-T9.1-04: Original chat data preserved after migration (rollback compatibility).
func TestT91_04_OriginalDataPreserved(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	seedChatData(t, app, 13, 2, 4)

	beforeChatSessions := countRows(t, app, "chat_sessions")
	beforeChatMessages := countRows(t, app, "chat_messages")

	applyChatMigration(t, app)

	// All un-deleted chat_sessions should have migrated_at set.
	var migratedCount int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM chat_sessions WHERE migrated_at IS NOT NULL",
	).Scan(&migratedCount)
	if err != nil {
		t.Fatalf("check migrated_at: %v", err)
	}
	// All chat_sessions in this test are un-deleted (is_deleted=0).
	if migratedCount != beforeChatSessions {
		t.Errorf("expected %d chat sessions with migrated_at set, got %d",
			beforeChatSessions, migratedCount)
	}

	// Original chat_messages still intact (not deleted).
	afterChatMessages := countRows(t, app, "chat_messages")
	if afterChatMessages != beforeChatMessages {
		t.Errorf("original chat messages not preserved: before=%d, after=%d",
			beforeChatMessages, afterChatMessages)
	}

	// Verify legacy sessions are NOT double-counted in the API (migrated_at filter works).
	resp := doRequest(t, app.Server, "GET", "/api/agent/sessions", testutil.StudentAToken(), nil)
	testutil.AssertStatus(t, resp, http.StatusOK)
	var sessions []dto.AgentSessionResponse
	json.NewDecoder(resp.Body).Decode(&sessions)
	resp.Body.Close()

	// Count seeded sessions in API response — should appear exactly once (not duplicated from legacy).
	seededCount := 0
	for _, s := range sessions {
		if s.Title == "Chat Session 0" || s.Title == "Chat Session 1" {
			seededCount++
		}
	}
	if seededCount != 2 {
		t.Errorf("expected exactly 2 seeded sessions in API (no duplicates), got %d", seededCount)
	}
}

// TEST-T9.1-05: Deleted sessions (is_deleted=1) are NOT migrated.
func TestT91_05_DeletedSessionsSkipped(t *testing.T) {
	app := testutil.SetupTestApp(t)
	_, err := testutil.BuildAgentFixture(context.Background(), app.DB)
	if err != nil {
		t.Fatalf("BuildAgentFixture: %v", err)
	}

	now := time.Now().Format(time.RFC3339)
	// Active session.
	_, err = app.DB.Writer.Exec(
		`INSERT INTO chat_sessions (id, student_id, title, is_deleted, created_at) VALUES (400, 13, 'Active', 0, ?)`, now)
	if err != nil {
		t.Fatalf("seed active: %v", err)
	}
	// Deleted session.
	_, err = app.DB.Writer.Exec(
		`INSERT INTO chat_sessions (id, student_id, title, is_deleted, created_at) VALUES (401, 13, 'Deleted', 1, ?)`, now)
	if err != nil {
		t.Fatalf("seed deleted: %v", err)
	}

	// Count un-deleted sessions before migration.
	var undeletedBefore int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM chat_sessions WHERE is_deleted = 0",
	).Scan(&undeletedBefore)
	if err != nil {
		t.Fatalf("count undeleted: %v", err)
	}

	beforeSessions := countRows(t, app, "agent_sessions")
	applyChatMigration(t, app)
	afterSessions := countRows(t, app, "agent_sessions")

	// Only un-deleted sessions should be migrated (deleted one is skipped).
	migratedDelta := afterSessions - beforeSessions
	if migratedDelta != undeletedBefore {
		t.Errorf("expected %d migrated sessions (all un-deleted), got %d",
			undeletedBefore, migratedDelta)
	}

	// Verify the deleted session was NOT migrated.
	var deletedInMapping int
	err = app.DB.Reader.QueryRow(
		"SELECT COUNT(*) FROM _chat_session_migration WHERE chat_session_id = 401",
	).Scan(&deletedInMapping)
	if err != nil {
		t.Fatalf("check mapping: %v", err)
	}
	if deletedInMapping != 0 {
		t.Error("deleted session was incorrectly migrated")
	}
}
