-- 001_initial.sql: Complete schema for training evaluation system
-- Adapted from PostgreSQL/SQLAlchemy to SQLite (TEXT for timestamps, JSON as TEXT, etc.)

-- ============================================================
-- Users
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'student' CHECK(role IN ('admin', 'teacher', 'student')),
    is_active INTEGER NOT NULL DEFAULT 1,
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until TEXT,
    last_login_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_users_username ON users(username);

-- ============================================================
-- Courses
-- ============================================================
CREATE TABLE IF NOT EXISTS courses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    is_archived INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Classes
-- ============================================================
CREATE TABLE IF NOT EXISTS classes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    course_id INTEGER NOT NULL REFERENCES courses(id),
    teacher_id INTEGER NOT NULL REFERENCES users(id),
    student_count INTEGER NOT NULL DEFAULT 0,
    is_archived INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Class Memberships
-- ============================================================
CREATE TABLE IF NOT EXISTS class_memberships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    student_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(class_id, student_id)
);
CREATE INDEX IF NOT EXISTS ix_class_memberships_class ON class_memberships(class_id);
CREATE INDEX IF NOT EXISTS ix_class_memberships_student ON class_memberships(student_id);

-- ============================================================
-- Training Tasks
-- ============================================================
CREATE TABLE IF NOT EXISTS training_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    requirements TEXT NOT NULL DEFAULT '',
    evaluation_criteria TEXT NOT NULL DEFAULT '',
    teacher_id INTEGER NOT NULL REFERENCES users(id),
    course_id INTEGER NOT NULL REFERENCES courses(id),
    status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft', 'published', 'closed')),
    deadline TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_training_tasks_teacher_status ON training_tasks(teacher_id, status);

-- ============================================================
-- Task-Class association (many-to-many)
-- ============================================================
CREATE TABLE IF NOT EXISTS task_classes (
    task_id INTEGER NOT NULL REFERENCES training_tasks(id) ON DELETE CASCADE,
    class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, class_id)
);

-- ============================================================
-- Dimensions (evaluation criteria per task)
-- ============================================================
CREATE TABLE IF NOT EXISTS dimensions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES training_tasks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    weight INTEGER NOT NULL CHECK(weight >= 1 AND weight <= 100),
    order_index INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS ix_dimensions_task_id ON dimensions(task_id);

-- ============================================================
-- Uploads
-- ============================================================
CREATE TABLE IF NOT EXISTS uploads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    student_id INTEGER NOT NULL REFERENCES users(id),
    filename TEXT NOT NULL,
    file_type TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    sha256 TEXT NOT NULL DEFAULT '',
    parse_status TEXT NOT NULL DEFAULT 'pending' CHECK(parse_status IN ('pending', 'parsing', 'parsed', 'failed')),
    version INTEGER NOT NULL DEFAULT 1,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_uploads_task_student ON uploads(task_id, student_id);
CREATE INDEX IF NOT EXISTS ix_uploads_status ON uploads(parse_status);
CREATE INDEX IF NOT EXISTS ix_uploads_sha256 ON uploads(student_id, sha256);

-- ============================================================
-- Parse Results
-- ============================================================
CREATE TABLE IF NOT EXISTS parse_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    upload_id INTEGER NOT NULL UNIQUE REFERENCES uploads(id) ON DELETE CASCADE,
    structured_content TEXT,
    raw_text TEXT NOT NULL DEFAULT '',
    simhash INTEGER,
    embedding TEXT,
    error_message TEXT NOT NULL DEFAULT '',
    parsed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Verify Results
-- ============================================================
CREATE TABLE IF NOT EXISTS verify_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    upload_id INTEGER NOT NULL UNIQUE REFERENCES uploads(id) ON DELETE CASCADE,
    match_rate REAL,
    checkpoints TEXT,
    missing_items TEXT,
    logic_issues TEXT,
    overall_confidence INTEGER,
    verified_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Evaluations
-- ============================================================
CREATE TABLE IF NOT EXISTS evaluations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    student_id INTEGER NOT NULL REFERENCES users(id),
    upload_id INTEGER NOT NULL REFERENCES uploads(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'scored', 'confirmed', 'rejected')),
    total_score REAL,
    teacher_comment TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Dimension Scores
-- ============================================================
CREATE TABLE IF NOT EXISTS dimension_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    evaluation_id INTEGER NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    dimension_id INTEGER NOT NULL REFERENCES dimensions(id),
    ai_score REAL,
    teacher_score REAL,
    rationale TEXT NOT NULL DEFAULT '',
    UNIQUE(evaluation_id, dimension_id)
);

-- ============================================================
-- Evaluation History (audit trail)
-- ============================================================
CREATE TABLE IF NOT EXISTS evaluation_histories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    evaluation_id INTEGER NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    operator_id INTEGER REFERENCES users(id),
    action TEXT NOT NULL,
    before_value TEXT,
    after_value TEXT,
    changed_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Similarity Records
-- ============================================================
CREATE TABLE IF NOT EXISTS similarity_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    upload_a_id INTEGER NOT NULL REFERENCES uploads(id),
    upload_b_id INTEGER NOT NULL REFERENCES uploads(id),
    hamming_distance INTEGER NOT NULL,
    cosine_similarity REAL,
    state TEXT NOT NULL DEFAULT 'suspect' CHECK(state IN ('suspect', 'confirmed', 'ignored')),
    reviewed_by INTEGER REFERENCES users(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    decided_at TEXT,
    CHECK(upload_a_id < upload_b_id),
    UNIQUE(task_id, upload_a_id, upload_b_id)
);
CREATE INDEX IF NOT EXISTS ix_similarity_task_state ON similarity_records(task_id, state);

-- ============================================================
-- LLM Configs
-- ============================================================
CREATE TABLE IF NOT EXISTS llm_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    chat_model TEXT NOT NULL,
    embed_model TEXT NOT NULL DEFAULT '',
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- System Config (runtime business parameters)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    description TEXT NOT NULL DEFAULT '',
    updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Audit Logs (append-only)
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
    user_id INTEGER,
    username TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    target TEXT NOT NULL DEFAULT '',
    result TEXT NOT NULL DEFAULT 'success',
    detail TEXT NOT NULL DEFAULT '',
    payload TEXT,
    client_ip TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    suspicious_flag INTEGER NOT NULL DEFAULT 0,
    ip TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_audit_logs_occurred_user ON audit_logs(occurred_at, user_id);
CREATE INDEX IF NOT EXISTS ix_audit_logs_suspicious ON audit_logs(suspicious_flag);

-- ============================================================
-- Notifications
-- ============================================================
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    payload TEXT,
    is_read INTEGER NOT NULL DEFAULT 0,
    link TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_notifications_recipient_read ON notifications(user_id, is_read, created_at);

-- ============================================================
-- Notification Preferences
-- ============================================================
CREATE TABLE IF NOT EXISTS notification_prefs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    event_type TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    UNIQUE(user_id, event_type)
);

-- ============================================================
-- Chat Sessions
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL REFERENCES users(id),
    evaluation_id INTEGER,
    title TEXT NOT NULL DEFAULT '新对话',
    is_deleted INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_active_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Chat Messages
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tool_call_id TEXT,
    tool_name TEXT,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS ix_chat_messages_session_created ON chat_messages(session_id, created_at);

-- ============================================================
-- Eval Templates
-- ============================================================
CREATE TABLE IF NOT EXISTS eval_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    visibility TEXT NOT NULL DEFAULT 'private' CHECK(visibility IN ('private', 'team', 'system')),
    owner_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    course_id INTEGER REFERENCES courses(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ============================================================
-- Template Dimensions
-- ============================================================
CREATE TABLE IF NOT EXISTS template_dimensions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id INTEGER NOT NULL REFERENCES eval_templates(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    weight INTEGER NOT NULL CHECK(weight >= 1 AND weight <= 100),
    order_index INTEGER NOT NULL DEFAULT 0
);

-- ============================================================
-- Import Jobs
-- ============================================================
CREATE TABLE IF NOT EXISTS import_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    operator_id INTEGER NOT NULL REFERENCES users(id),
    job_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    failed_file_path TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS ix_import_jobs_operator_created ON import_jobs(operator_id, created_at);

-- ============================================================
-- Import Records
-- ============================================================
CREATE TABLE IF NOT EXISTS import_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    row_number INTEGER NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT ''
);

-- ============================================================
-- Student Profiles
-- ============================================================
CREATE TABLE IF NOT EXISTS student_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id INTEGER NOT NULL UNIQUE REFERENCES users(id),
    radar_data TEXT,
    weakness_list TEXT,
    suggestions TEXT,
    score_trend TEXT,
    source_evaluation_count INTEGER NOT NULL DEFAULT 0,
    computed_at TEXT NOT NULL DEFAULT (datetime('now'))
);
