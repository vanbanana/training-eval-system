-- 004_chat_migration.sql: Migrate legacy chat_sessions/chat_messages to agent_sessions/agent_messages.
--
-- This migration is:
--   - Idempotent: safe to run multiple times (only migrates un-migrated sessions).
--   - Non-destructive: original chat_sessions and chat_messages are preserved.
--   - Reversible: see cmd/rollback_chat for the rollback procedure.
--
-- After migration, legacy sessions are still readable through the original
-- chat_sessions/chat_messages tables; the agent_repo legacy methods simply
-- skip sessions that have already been migrated (WHERE migrated_at IS NULL).

-- Step 1: Add migrated_at column to chat_sessions (tracks which sessions have been migrated).
ALTER TABLE chat_sessions ADD COLUMN migrated_at TEXT;

-- Step 2: Create permanent mapping table (survives migration, used by rollback).
CREATE TABLE IF NOT EXISTS _chat_session_migration (
    chat_session_id INTEGER PRIMARY KEY,
    agent_session_id INTEGER NOT NULL
);

-- Step 3: Migrate un-migrated, non-deleted chat sessions → agent_sessions.
--   student_id → owner_id (role hardcoded to 'student')
--   evaluation_id preserved in context_json
INSERT INTO agent_sessions (owner_id, owner_role, agent_role, title, context_json, created_at, last_active_at)
SELECT
    cs.student_id,
    'student',
    'student',
    cs.title,
    CASE
        WHEN cs.evaluation_id IS NOT NULL
        THEN '{"_migrated_from_chat_id":' || cs.id || ',"evaluation_id":' || cs.evaluation_id || '}'
        ELSE '{"_migrated_from_chat_id":' || cs.id || '}'
    END,
    cs.created_at,
    cs.last_active_at
FROM chat_sessions cs
WHERE cs.is_deleted = 0
  AND cs.migrated_at IS NULL;

-- Step 4: Record the old → new session ID mapping.
-- Agent sessions are identified by the context_json marker set in Step 3.
INSERT OR IGNORE INTO _chat_session_migration (chat_session_id, agent_session_id)
SELECT cs.id, asent.id
FROM chat_sessions cs
JOIN agent_sessions asent
    ON asent.context_json LIKE '%"_' || '_migrated_from_chat_id":' || cs.id || '%';

-- Step 5: Migrate chat_messages → agent_messages for mapped sessions.
INSERT INTO agent_messages (session_id, role, content, tool_call_id, tool_name, prompt_tokens, completion_tokens, created_at)
SELECT
    csm.agent_session_id,
    cm.role,
    cm.content,
    cm.tool_call_id,
    cm.tool_name,
    cm.prompt_tokens,
    cm.completion_tokens,
    cm.created_at
FROM chat_messages cm
JOIN _chat_session_migration csm ON csm.chat_session_id = cm.session_id
WHERE NOT EXISTS (
    SELECT 1 FROM _chat_session_migration WHERE chat_session_id = cm.session_id AND agent_session_id = 0
);

-- Step 6: Mark migrated chat sessions.
UPDATE chat_sessions SET migrated_at = datetime('now') WHERE migrated_at IS NULL AND is_deleted = 0;
