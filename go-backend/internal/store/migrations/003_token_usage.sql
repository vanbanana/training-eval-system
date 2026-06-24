-- 003_token_usage.sql: Token usage tracking for AI agent calls (T8.3).

CREATE TABLE IF NOT EXISTS token_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    user_role TEXT NOT NULL,
    agent_role TEXT NOT NULL,
    session_id INTEGER NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    success INTEGER NOT NULL DEFAULT 1,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    cost_status TEXT NOT NULL DEFAULT 'unknown',
    estimated_cost REAL NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_token_usage_user ON token_usage(user_id);
CREATE INDEX IF NOT EXISTS ix_token_usage_created ON token_usage(created_at);
CREATE INDEX IF NOT EXISTS ix_token_usage_role ON token_usage(user_role);
CREATE INDEX IF NOT EXISTS ix_token_usage_session ON token_usage(session_id);
