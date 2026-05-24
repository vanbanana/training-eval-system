# Requirements Document

## Introduction

本需求描述将现有 Python（FastAPI）后端 1:1 重写为 Go 语言实现的技术要求。重写目标是生成单一静态 ELF 二进制文件，零运行时依赖，可交叉编译至 GOOS=linux GOARCH=loong64（龙芯 LoongArch），同时保持与现有 Vue 3 前端完全兼容的 API 契约。

核心技术变更：
- Web 框架：FastAPI → Go chi v5 + net/http
- 数据库：PostgreSQL → SQLite（modernc.org/sqlite，纯 Go）
- 异步任务：Celery + Redis → goroutine + channel
- 实时推送：WebSocket → SSE（Server-Sent Events）
- 向量相似度：pgvector → 纯 Go 余弦相似度计算
- 缓存/队列：Redis → 进程内 Go 数据结构
- 加密：cryptography (Python) → Go crypto/aes (AES-256-GCM)
- 认证：python-jose → Go crypto (HMAC-SHA256 JWT)

重写范围覆盖现有后端全部 20+ 路由组、所有业务逻辑（评分计算、相似度检测、画像生成、Function Calling 编排）及静态文件服务。

## Glossary

- **Go_Backend**: 使用 Go 语言重写的后端服务，输出为单一静态链接 ELF 二进制文件
- **API_Contract**: 现有 Python 后端定义的 HTTP 请求/响应 JSON 格式规范，前端依赖此契约
- **Chi_Router**: go-chi/chi v5 路由库，用于 HTTP 路由与中间件
- **SQLite_Store**: 使用 modernc.org/sqlite（纯 Go 实现）的 SQLite 数据库存储层
- **SSE_Module**: 基于 Server-Sent Events 协议的实时推送模块，替代原 WebSocket 实现
- **LLM_Client**: 通过 net/http 调用云端 OpenAI 兼容 API 的 HTTP 客户端
- **Task_Runner**: 基于 goroutine 和 channel 的异步任务执行引擎，替代 Celery
- **Similarity_Engine**: 纯 Go 实现的文本相似度检测引擎（SimHash + 余弦相似度）
- **Static_Server**: Go 二进制内嵌或外部 dist/ 目录的静态文件服务模块
- **Config_Module**: 基于环境变量和 .env 文件的配置加载模块
- **Crypto_Module**: 基于 Go crypto/aes 的 AES-256-GCM 加密模块
- **Auth_Module**: 基于 Go crypto/hmac 的 JWT 认证与 RBAC 鉴权模块
- **Binary**: 交叉编译产出的单一静态链接 ELF 可执行文件

## Requirements

### Requirement 1: API 契约兼容性

**User Story:** As a frontend developer, I want the Go backend to expose identical API endpoints with the same request/response JSON format, so that the Vue 3 frontend works without any modification.

#### Acceptance Criteria

1. THE Go_Backend SHALL expose all HTTP endpoints matching the Python backend route paths, HTTP methods, and URL parameter names exactly
2. THE Go_Backend SHALL accept request JSON bodies with identical field names, types, and nesting structure as the Python backend for every endpoint
3. THE Go_Backend SHALL return response JSON bodies with identical field names, types, nesting structure, and null/empty value semantics as the Python backend for every endpoint
4. THE Go_Backend SHALL return identical HTTP status codes for success and error conditions as the Python backend (200, 201, 400, 401, 403, 404, 409, 422, 500)
5. THE Go_Backend SHALL return error responses in the same JSON structure as the Python backend, containing "detail" field with error message string or validation error array
6. WHEN the frontend sends a paginated list request, THE Go_Backend SHALL accept the same query parameters (page, page_size, search, sort, order) and return the same pagination envelope (items, total, page, page_size)
7. THE Go_Backend SHALL preserve the same URL prefix structure: /api/auth/*, /api/users/*, /api/tasks/*, /api/uploads/*, /api/evaluations/*, /api/courses/*, /api/classes/*, /api/llm/*, /api/audit/*, /api/profiles/*, /api/reports/*, /api/notifications/*, /api/chat/*, /api/similarity/*, /api/templates/*, /api/imports/*, /api/dashboard/*, /api/parse/*, /api/grading/*, /api/account/*

### Requirement 2: 构建与部署

**User Story:** As a system administrator, I want the Go backend to compile into a single static binary with zero runtime dependencies, so that deployment on LoongArch servers requires only copying one file.

#### Acceptance Criteria

1. THE Go_Backend SHALL compile to a single statically-linked ELF binary using CGO_ENABLED=0
2. THE Go_Backend SHALL cross-compile successfully with GOOS=linux GOARCH=loong64 targeting LoongArch architecture
3. THE Binary SHALL start and serve HTTP requests without requiring any shared libraries, interpreters, or runtime environments on the target system
4. THE Go_Backend SHALL use Go 1.25 as the minimum compiler version
5. THE Go_Backend SHALL use only pure-Go dependencies with zero CGO requirements
6. THE Go_Backend SHALL serve the frontend static files from a dist/ directory located relative to the binary, configurable via environment variable
7. WHEN the dist/ directory exists and contains index.html, THE Static_Server SHALL serve frontend assets and fall back to index.html for client-side routing paths
8. THE Go_Backend SHALL start and become ready to accept HTTP requests within 5 seconds on the target hardware (4-core LoongArch, 8GB RAM)
9. THE Go_Backend source code SHALL reside in a go-backend/ directory at the project root, separate from the existing Python backend

### Requirement 3: 数据库层（SQLite）

**User Story:** As a system administrator, I want the backend to use SQLite instead of PostgreSQL, so that the deployment has zero external database dependencies.

#### Acceptance Criteria

1. THE SQLite_Store SHALL use modernc.org/sqlite as the database driver, which is a pure-Go SQLite implementation requiring zero CGO
2. THE SQLite_Store SHALL store all application data (users, tasks, uploads, evaluations, courses, classes, notifications, audit logs, chat history, similarity records, profiles, templates, import jobs, system config) in a single SQLite database file
3. THE SQLite_Store SHALL apply WAL (Write-Ahead Logging) mode for concurrent read performance
4. THE SQLite_Store SHALL use a connection pool with configurable maximum connections (default 1 writer, multiple readers)
5. THE SQLite_Store SHALL implement database schema migrations that create all required tables on first startup
6. THE SQLite_Store SHALL adapt the PostgreSQL schema to SQLite-compatible types: JSONB → TEXT (JSON string), UUID → TEXT, TIMESTAMP WITH TIME ZONE → TEXT (RFC3339), ARRAY → TEXT (JSON array), SERIAL → INTEGER PRIMARY KEY AUTOINCREMENT
7. THE SQLite_Store SHALL store embedding vectors as JSON arrays of float64 in TEXT columns
8. IF the database file does not exist on startup, THEN THE SQLite_Store SHALL create it and run all migrations automatically
9. THE SQLite_Store SHALL support database file path configuration via environment variable (default: ./data/app.db)
10. THE SQLite_Store SHALL implement row-level locking semantics using SQLite's built-in transaction isolation for concurrent write safety

### Requirement 4: 认证与授权

**User Story:** As a user, I want the Go backend to provide the same authentication and authorization behavior as the Python backend, so that my login credentials and permissions work identically.

#### Acceptance Criteria

1. THE Auth_Module SHALL implement JWT-based authentication using HMAC-SHA256 (HS256) signing with the same secret key format as the Python backend
2. THE Auth_Module SHALL generate access tokens with the same payload structure (sub, role, exp, iat) and expiration semantics as the Python backend
3. THE Auth_Module SHALL validate passwords using bcrypt with cost factor 10, compatible with hashes generated by the Python backend's passlib
4. THE Auth_Module SHALL enforce the same RBAC rules: admin, teacher, student roles with identical permission boundaries per endpoint
5. WHEN a user submits incorrect credentials 5 consecutive times, THE Auth_Module SHALL lock the account for 15 minutes, matching the Python backend behavior
6. IF a user session has been inactive for 30 minutes, THEN THE Auth_Module SHALL reject subsequent requests with HTTP 401 and require re-authentication
7. THE Auth_Module SHALL accept the Authorization header in "Bearer {token}" format and validate tokens on every protected endpoint
8. THE Auth_Module SHALL return HTTP 401 for missing/invalid tokens and HTTP 403 for insufficient role permissions, with the same error response JSON structure

### Requirement 5: 异步任务引擎

**User Story:** As a developer, I want the Go backend to process long-running tasks (parsing, evaluation, similarity detection) asynchronously using goroutines, so that HTTP requests return immediately without blocking.

#### Acceptance Criteria

1. THE Task_Runner SHALL execute long-running operations (file parsing, verification, evaluation scoring, similarity detection, profile generation) in background goroutines, returning task IDs to the caller immediately
2. THE Task_Runner SHALL use buffered channels for task queuing with configurable buffer size (default: 100)
3. THE Task_Runner SHALL limit concurrent task execution using a worker pool with configurable concurrency (default: 4 workers)
4. THE Task_Runner SHALL track task status (pending, running, completed, failed) in the SQLite database, queryable via API
5. WHEN a task completes or fails, THE Task_Runner SHALL update the task status in the database and emit an SSE event to connected clients
6. IF a task panics during execution, THEN THE Task_Runner SHALL recover the panic, mark the task as failed with the panic message, and continue processing other tasks
7. THE Task_Runner SHALL support task retry with configurable maximum attempts (default: 3) and exponential backoff (1s, 2s, 4s)
8. THE Task_Runner SHALL gracefully drain all in-flight tasks on application shutdown, waiting up to 30 seconds before force-terminating

### Requirement 6: 实时推送（SSE）

**User Story:** As a frontend developer, I want the Go backend to provide Server-Sent Events for real-time updates, so that the frontend receives parsing progress, notifications, and chat streaming without polling.

#### Acceptance Criteria

1. THE SSE_Module SHALL expose an SSE endpoint at GET /api/sse/events that the frontend connects to for real-time updates
2. THE SSE_Module SHALL authenticate SSE connections using the same JWT token (passed as query parameter or Authorization header)
3. THE SSE_Module SHALL multiplex multiple event types over a single connection: progress, notification, chat_stream, task_status
4. WHEN a parsing task updates its progress, THE SSE_Module SHALL emit a "progress" event with task_id and percentage (0-100) to the owning user's connection
5. WHEN a notification is created for a user, THE SSE_Module SHALL emit a "notification" event with the notification payload to that user's connection
6. WHEN the LLM streams a chat response, THE SSE_Module SHALL emit "chat_stream" events with incremental text chunks to the requesting user's connection
7. THE SSE_Module SHALL send keepalive comments (": keepalive") every 30 seconds to prevent connection timeout
8. IF a client disconnects, THEN THE SSE_Module SHALL clean up the connection resources and stop sending events for that client
9. THE SSE_Module SHALL support multiple concurrent connections per user (e.g., multiple browser tabs)

### Requirement 7: LLM 客户端

**User Story:** As a system administrator, I want the Go backend to call cloud LLM APIs using the same OpenAI-compatible protocol, so that all AI features (parsing, verification, evaluation, chat, profiles) work identically.

#### Acceptance Criteria

1. THE LLM_Client SHALL call cloud LLM APIs using the OpenAI Chat Completions protocol over HTTPS via net/http
2. THE LLM_Client SHALL support configurable base URL, API key, chat model name, and embedding model name, loaded from the database (hot-swappable without restart)
3. THE LLM_Client SHALL implement streaming responses (SSE from LLM API) for chat completions, forwarding chunks to the SSE_Module
4. THE LLM_Client SHALL implement the embeddings endpoint for generating text vectors used in similarity detection
5. THE LLM_Client SHALL implement Function Calling (tool_calls in request/response) for the chat assistant feature, matching the Python backend's tool definitions
6. THE LLM_Client SHALL implement exponential backoff retry (1s, 2s, 4s) with maximum 3 attempts on transient failures (HTTP 429, 500, 502, 503, 504, timeout)
7. THE LLM_Client SHALL implement circuit breaker logic: after 5 consecutive failures, reject requests for 30 seconds before attempting half-open recovery
8. THE LLM_Client SHALL log every LLM call with: model name, endpoint, duration_ms, prompt_tokens, completion_tokens, success/failure status
9. THE LLM_Client SHALL encrypt stored API keys using AES-256-GCM via the Crypto_Module, decrypting only at call time
10. IF the LLM service is unavailable (circuit open), THEN THE LLM_Client SHALL return a structured error allowing the API layer to respond with "service unavailable" and enable manual-only mode

### Requirement 8: 相似度检测引擎

**User Story:** As a teacher, I want the Go backend to detect similarity between student submissions using the same dual-engine approach, so that plagiarism detection works identically to the Python version.

#### Acceptance Criteria

1. THE Similarity_Engine SHALL implement SimHash fingerprinting in pure Go for fast text similarity pre-screening
2. THE Similarity_Engine SHALL implement cosine similarity calculation in pure Go over embedding vectors stored as float64 arrays
3. THE Similarity_Engine SHALL use the same two-phase detection strategy: SimHash for coarse filtering (hamming distance threshold), then cosine similarity for precise ranking on candidates that pass the coarse filter
4. THE Similarity_Engine SHALL compare each new submission against all other submissions within the same training task scope only
5. THE Similarity_Engine SHALL output a similarity score (0-100%) and references to the top 3 most similar submissions, matching the Python backend's response format
6. IF the similarity score between any two submissions exceeds 80%, THEN THE Similarity_Engine SHALL flag the pair as "suspected similar" in the database
7. THE Similarity_Engine SHALL store computed SimHash fingerprints and embedding vectors in the SQLite database for incremental comparison without recomputation
8. THE Similarity_Engine SHALL use configurable thresholds for SimHash hamming distance and cosine similarity score, loaded from system configuration

### Requirement 9: 加密与安全

**User Story:** As a system administrator, I want the Go backend to provide the same security guarantees (encrypted API keys, secure password storage, input validation), so that the system remains secure after the rewrite.

#### Acceptance Criteria

1. THE Crypto_Module SHALL implement AES-256-GCM encryption/decryption using Go's crypto/aes and crypto/cipher packages for API key storage
2. THE Crypto_Module SHALL load the master encryption key from the environment variable (same variable name as Python backend), not from code or database
3. THE Auth_Module SHALL hash passwords using bcrypt with cost factor 10 via golang.org/x/crypto/bcrypt, producing hashes compatible with the Python backend's existing stored hashes
4. THE Go_Backend SHALL validate all user input (request bodies, query parameters, path parameters) before processing, returning HTTP 422 with field-level error details for invalid input
5. THE Go_Backend SHALL sanitize file names and reject path traversal attempts in upload endpoints
6. THE Go_Backend SHALL set secure HTTP headers: X-Content-Type-Options: nosniff, X-Frame-Options: DENY, Content-Security-Policy for static files
7. THE Go_Backend SHALL implement CORS middleware with configurable allowed origins, matching the Python backend's CORS configuration
8. THE Go_Backend SHALL rate-limit authentication endpoints to prevent brute-force attacks (configurable, default: 10 attempts per IP per minute)

### Requirement 10: 配置管理

**User Story:** As a system administrator, I want the Go backend to load configuration from environment variables and .env files with the same parameter names, so that existing deployment scripts work without modification.

#### Acceptance Criteria

1. THE Config_Module SHALL load configuration from environment variables with the TES_ prefix, matching the Python backend's pydantic-settings convention
2. THE Config_Module SHALL support loading from a .env file when present in the working directory or at a path specified by TES_ENV_FILE
3. THE Config_Module SHALL provide the following configuration parameters with sensible defaults: TES_DB_PATH (./data/app.db), TES_UPLOAD_ROOT (./data/uploads), TES_JWT_SECRET (required), TES_LLM_KEY_MASTER (required), TES_CORS_ORIGINS (*), TES_ENV (prod), TES_LISTEN_ADDR (:8000), TES_DIST_DIR (./dist), TES_MAX_UPLOAD_SIZE_MB (50), TES_WORKER_COUNT (4), TES_TASK_BUFFER_SIZE (100)
4. THE Config_Module SHALL support runtime business configuration stored in the SQLite system_config table (similarity thresholds, scoring weights, chat quotas), hot-reloadable without restart
5. IF a required configuration parameter is missing on startup, THEN THE Go_Backend SHALL exit with a clear error message indicating which parameter is missing
6. THE Config_Module SHALL never log or expose secret values (JWT secret, master key, API keys) in any output

### Requirement 11: 日志与可观测性

**User Story:** As a system administrator, I want the Go backend to produce structured JSON logs with trace IDs, so that I can monitor and debug the system effectively.

#### Acceptance Criteria

1. THE Go_Backend SHALL output structured JSON logs to stdout, one JSON object per line
2. THE Go_Backend SHALL include the following fields in every log entry: timestamp (RFC3339), level (info/warn/error), trace_id, message, and context-specific fields
3. THE Go_Backend SHALL generate a unique trace_id for each HTTP request (or inherit from X-Trace-Id header) and propagate it through all downstream operations including goroutine tasks
4. THE Go_Backend SHALL log at request entry (method, path, content_length) and exit (status_code, duration_ms) for every HTTP request
5. THE Go_Backend SHALL log all LLM API calls with: endpoint, model, duration_ms, tokens, success/failure
6. THE Go_Backend SHALL log all errors with: error_type, message, stack trace (for panics), and request context
7. THE Go_Backend SHALL filter sensitive fields (password, api_key, secret, token) from log output, replacing values with "***"
8. THE Go_Backend SHALL support configurable log level via TES_LOG_LEVEL environment variable (debug, info, warn, error)

### Requirement 12: 文件上传与存储

**User Story:** As a student, I want the Go backend to handle file uploads with the same validation and storage behavior, so that my submission workflow works identically.

#### Acceptance Criteria

1. THE Go_Backend SHALL accept multipart file uploads with the same size limits (1KB minimum, 50MB maximum per file, configurable)
2. THE Go_Backend SHALL validate uploaded files using both extension whitelist (.doc, .docx, .pdf, .png, .jpg, .jpeg) and magic number (file header) verification
3. THE Go_Backend SHALL store uploaded files in the configured upload directory with the same directory structure as the Python backend (organized by task_id/user_id/)
4. THE Go_Backend SHALL compute SHA-256 checksums for uploaded files and store them in the database for integrity verification
5. THE Go_Backend SHALL support chunked/resumable uploads matching the Python backend's tus-compatible or custom chunked upload protocol
6. IF an uploaded file fails validation (wrong type, too large, too small, magic number mismatch), THEN THE Go_Backend SHALL reject the upload with HTTP 422 and a descriptive error message matching the Python backend's error format
7. WHILE a training task is in "closed" status, THE Go_Backend SHALL reject new file uploads for that task with HTTP 403

### Requirement 13: 业务逻辑保真

**User Story:** As a teacher, I want all business logic (evaluation scoring, weight calculation, profile generation, grading workflow) to produce identical results in the Go backend, so that academic outcomes are consistent.

#### Acceptance Criteria

1. THE Go_Backend SHALL implement the same weighted scoring formula: dimension scores × dimension weights, with weights summing to 100% and each weight minimum 5%
2. THE Go_Backend SHALL implement the same composite score calculation: (objective_score × objective_ratio + subjective_score × subjective_ratio), with configurable ratios (default 60%/40%), result rounded to one decimal place
3. THE Go_Backend SHALL implement the same task state machine: draft → published → closed, with identical transition rules and side effects
4. THE Go_Backend SHALL implement the same grading workflow states: unsubmitted → submitted → parsed → scored → confirmed → returned, with identical transition rules
5. THE Go_Backend SHALL implement the same account lockout logic: 5 failed attempts → 15 minute lock, with counter reset on successful login
6. THE Go_Backend SHALL implement the same notification triggers: task published, parse complete, evaluation complete, submission returned, weakness analysis updated, deadline reminder (24h before)
7. THE Go_Backend SHALL implement the same chat assistant behavior: context injection (task requirements, submission summary, scores), 20-round limit per session, 500-character input limit, 50 calls per student per day
8. THE Go_Backend SHALL implement the same audit logging: capturing identical event types, fields, and immutability guarantees (append-only, no update/delete via API)
9. THE Go_Backend SHALL implement the same dashboard aggregation logic per role (admin/teacher/student) with identical card data and statistics

### Requirement 14: 健康检查与优雅关闭

**User Story:** As a system administrator, I want the Go backend to support health checks and graceful shutdown, so that I can integrate it with process managers and load balancers.

#### Acceptance Criteria

1. THE Go_Backend SHALL expose GET /healthz endpoint returning {"status": "ok", "env": "<current_env>"} with HTTP 200 when the service is healthy
2. THE Go_Backend SHALL verify database connectivity in the health check and return HTTP 503 with {"status": "unhealthy", "reason": "..."} if the database is unreachable
3. WHEN the process receives SIGTERM or SIGINT, THE Go_Backend SHALL stop accepting new connections, drain in-flight HTTP requests (up to 15 seconds), drain in-flight background tasks (up to 30 seconds), then exit cleanly
4. THE Go_Backend SHALL log shutdown initiation and completion events with duration
5. IF graceful shutdown exceeds the timeout, THEN THE Go_Backend SHALL force-terminate remaining operations and exit with a non-zero exit code

### Requirement 15: 数据库备份

**User Story:** As a system administrator, I want the Go backend to support SQLite database backup, so that I can recover data in case of failure.

#### Acceptance Criteria

1. THE Go_Backend SHALL support triggering a database backup via admin API endpoint (POST /api/admin/backup)
2. THE Go_Backend SHALL implement SQLite online backup (using the backup API or file copy with WAL checkpoint) to create consistent backup files
3. THE Go_Backend SHALL store backup files in a configurable directory (default: ./data/backups/) with timestamp-based filenames
4. THE Go_Backend SHALL support automatic scheduled backups at configurable intervals (default: every 24 hours)
5. THE Go_Backend SHALL retain backup files for a configurable number of days (default: 7 days), automatically deleting older backups
6. IF a backup operation fails, THEN THE Go_Backend SHALL log the error and emit an admin notification, without affecting normal service operation

### Requirement 16: 路由完整性

**User Story:** As a developer, I want a complete mapping of all Python backend routes to Go implementations, so that no functionality is lost in the rewrite.

#### Acceptance Criteria

1. THE Go_Backend SHALL implement all authentication routes: POST /api/auth/login, POST /api/auth/logout, POST /api/auth/refresh, GET /api/auth/me
2. THE Go_Backend SHALL implement all user management routes: GET/POST /api/users, GET/PUT/DELETE /api/users/{id}, PUT /api/users/{id}/toggle-status
3. THE Go_Backend SHALL implement all task management routes: GET/POST /api/tasks, GET/PUT/DELETE /api/tasks/{id}, POST /api/tasks/{id}/publish, POST /api/tasks/{id}/close, and task editing sub-routes
4. THE Go_Backend SHALL implement all upload routes: GET/POST /api/uploads, GET/DELETE /api/uploads/{id}, POST /api/uploads/{id}/retry-parse
5. THE Go_Backend SHALL implement all evaluation routes: GET/POST /api/evaluations, GET/PUT /api/evaluations/{id}, POST /api/evaluations/batch-confirm
6. THE Go_Backend SHALL implement all grading routes: GET /api/grading/tasks/{id}/submissions, GET/PUT /api/grading/submissions/{id}, POST /api/grading/submissions/{id}/return
7. THE Go_Backend SHALL implement all course and class routes: CRUD for /api/courses and /api/classes, including member management
8. THE Go_Backend SHALL implement all LLM configuration routes: GET/PUT /api/llm/config, POST /api/llm/test-connection
9. THE Go_Backend SHALL implement all auxiliary routes: /api/audit/*, /api/profiles/*, /api/reports/*, /api/notifications/*, /api/chat/*, /api/similarity/*, /api/templates/*, /api/imports/*, /api/dashboard/*, /api/parse/*, /api/account/*
10. THE Go_Backend SHALL implement the SSE endpoint replacing WebSocket: GET /api/sse/events
11. WHEN TES_ENV is "dev" or "test", THE Go_Backend SHALL expose /api/_dev/* debug endpoints matching the Python backend's development helpers

### Requirement 17: 进程内状态管理

**User Story:** As a developer, I want the Go backend to use in-process data structures instead of Redis for caching and pub/sub, so that the deployment has zero external service dependencies.

#### Acceptance Criteria

1. THE Go_Backend SHALL implement an in-process LRU cache for system configuration values with configurable TTL (default: 60 seconds)
2. THE Go_Backend SHALL implement an in-process pub/sub mechanism for SSE event distribution to connected clients
3. THE Go_Backend SHALL implement in-process rate limiting using token bucket or sliding window algorithm for authentication and API throttling
4. THE Go_Backend SHALL implement in-process mutex/semaphore for write serialization where the Python backend uses Redis distributed locks
5. THE Go_Backend SHALL implement in-process task queue using buffered Go channels, replacing Celery's Redis-backed queue
6. THE Go_Backend SHALL ensure all in-process state is thread-safe using appropriate Go synchronization primitives (sync.Mutex, sync.RWMutex, atomic operations, channels)
7. THE Go_Backend SHALL clear expired cache entries periodically (configurable interval, default: 5 minutes) to prevent memory growth

### Requirement 18: 报表生成

**User Story:** As a teacher, I want the Go backend to generate PDF and Excel reports with the same content and structure, so that report exports work identically.

#### Acceptance Criteria

1. THE Go_Backend SHALL generate PDF reports containing evaluation details, dimension scores, comments, and improvement suggestions, with Chinese text rendering support
2. THE Go_Backend SHALL generate Excel (.xlsx) reports containing statistical data (averages, medians, standard deviations, score distributions) matching the Python backend's output format
3. THE Go_Backend SHALL generate reports within 30 seconds for class sizes up to 100 students
4. THE Go_Backend SHALL support the same report filtering options: time range, class, course, teacher
5. IF a student has incomplete evaluation data, THEN THE Go_Backend SHALL mark those dimensions as "未评价" in the report and include available data
6. THE Go_Backend SHALL use pure-Go libraries for PDF and Excel generation (no CGO dependencies)

### Requirement 19: 文档解析集成

**User Story:** As a developer, I want the Go backend to parse uploaded documents (Word, PDF, images) using local pure-Go libraries for text extraction and cloud multimodal LLM API for OCR/image recognition, so that the parsing pipeline works without any local OCR dependencies.

#### Acceptance Criteria

1. THE Go_Backend SHALL extract text content from .docx files using Go standard library (archive/zip + encoding/xml) to parse OOXML word/document.xml
2. THE Go_Backend SHALL extract text content from .pdf files using ledongthuc/pdf (pure-Go PDF text extraction library, verified for Chinese content)
3. WHEN local PDF text extraction yields fewer than 50 characters per page, THE Go_Backend SHALL treat the PDF as image-based and delegate to the multimodal LLM API for OCR
4. THE Go_Backend SHALL delegate all image OCR (.png, .jpg) and scanned PDF recognition to the cloud multimodal LLM API (same provider as chat/evaluation), sending base64-encoded page images for text extraction
5. THE Go_Backend SHALL NOT include any local OCR engine (no Tesseract, no CGO-based OCR library) — all OCR is handled by the cloud LLM API
6. THE Go_Backend SHALL produce the same structured parse output format (title hierarchy, paragraphs, figure descriptions) as the Python backend
7. WHEN parsing completes, THE Go_Backend SHALL update the upload record status and emit an SSE progress event, matching the Python backend's behavior
8. IF parsing fails or times out (configurable, default 120 seconds), THEN THE Go_Backend SHALL mark the upload as parse-failed, log the error, and allow manual retry via API
9. THE Go_Backend SHALL support concurrent parsing of multiple uploads limited by the Task_Runner's worker pool

