# Design Document

## Overview

本设计将现有 Python FastAPI 后端 1:1 重写为 Go 语言实现，采用 Clean Architecture 分层，输出单一静态链接 ELF 二进制文件，零 CGO 依赖，可交叉编译至 LoongArch (loong64)。所有 19 项需求映射到具体 Go 包和接口，保持与 Vue 3 前端完全兼容的 API 契约。

## Architecture

采用 Clean Architecture 分层（handler → service → repository → store），输出单一静态链接 ELF 二进制文件，零 CGO 依赖，可交叉编译至 LoongArch (loong64)。

### 技术栈

| 层 | 技术选型 |
|---|---|
| HTTP 框架 | go-chi/chi v5 + net/http |
| 数据库 | modernc.org/sqlite (纯 Go) + database/sql |
| 认证 | crypto/hmac (JWT HS256) + golang.org/x/crypto/bcrypt |
| 加密 | crypto/aes + crypto/cipher (AES-256-GCM) |
| 异步任务 | goroutine worker pool + buffered channels |
| 实时推送 | net/http SSE streaming |
| LLM 调用 | net/http client + SSE response parsing |
| 相似度 | SimHash (纯 Go) + cosine similarity (纯 Go) |
| PDF 生成 | jung-kurt/gofpdf 或 go-pdf/fpdf |
| Excel 生成 | excelize (纯 Go) |
| DOCX 解析 | 纯 Go OOXML |
| PDF 提取 | ledongthuc/pdf 或 pdfcpu |
| 配置 | godotenv + os.Getenv (TES_ 前缀) |
| 日志 | log/slog (Go 1.21+ stdlib) |
| 编译器 | Go 1.25+ |

### 架构分层

```
┌─────────────────────────────────────────────────┐
│  HTTP Layer (chi router + middleware)            │
│  handler/ — 路由注册、请求解析、响应序列化        │
├─────────────────────────────────────────────────┤
│  Service Layer                                   │
│  service/ — 业务编排、事务边界、规则校验          │
├─────────────────────────────────────────────────┤
│  Repository Layer                                │
│  repository/ — 数据访问接口 + SQL 实现           │
├─────────────────────────────────────────────────┤
│  Store Layer                                     │
│  store/ — database/sql 连接池、迁移、事务管理     │
├─────────────────────────────────────────────────┤
│  Infrastructure                                  │
│  内存缓存、pub/sub、worker pool、LLM client      │
└─────────────────────────────────────────────────┘
```

## Project Structure

```
go-backend/
├── cmd/
│   └── server/
│       └── main.go              # 入口：配置加载、依赖注入、启动 HTTP server
├── internal/
│   ├── config/
│   │   └── config.go            # TES_ 前缀环境变量 + .env 加载
│   ├── handler/
│   │   ├── auth.go              # /api/auth/*
│   │   ├── users.go             # /api/users/*
│   │   ├── tasks.go             # /api/tasks/*
│   │   ├── uploads.go           # /api/uploads/*
│   │   ├── evaluations.go       # /api/evaluations/*
│   │   ├── grading.go           # /api/grading/*
│   │   ├── courses.go           # /api/courses/*
│   │   ├── classes.go           # /api/classes/*
│   │   ├── llm.go               # /api/llm/*
│   │   ├── audit.go             # /api/audit/*
│   │   ├── profiles.go          # /api/profiles/*
│   │   ├── reports.go           # /api/reports/*
│   │   ├── notifications.go     # /api/notifications/*
│   │   ├── chat.go              # /api/chat/*
│   │   ├── similarity.go        # /api/similarity/*
│   │   ├── templates.go         # /api/templates/*
│   │   ├── imports.go           # /api/imports/*
│   │   ├── dashboard.go         # /api/dashboard/*
│   │   ├── parse.go             # /api/parse/*
│   │   ├── account.go           # /api/account/*
│   │   ├── sse.go               # /api/sse/events
│   │   ├── health.go            # /healthz
│   │   ├── admin.go             # /api/admin/* (backup)
│   │   ├── dev.go               # /api/_dev/* (dev/test only)
│   │   ├── static.go            # 静态文件服务 + SPA fallback
│   │   ├── middleware.go        # 中间件注册
│   │   └── router.go            # chi 路由总装配
│   ├── service/
│   │   ├── auth_service.go
│   │   ├── user_service.go
│   │   ├── task_service.go
│   │   ├── upload_service.go
│   │   ├── evaluation_service.go
│   │   ├── grading_service.go
│   │   ├── course_service.go
│   │   ├── class_service.go
│   │   ├── llm_config_service.go
│   │   ├── audit_service.go
│   │   ├── profile_service.go
│   │   ├── report_service.go
│   │   ├── notification_service.go
│   │   ├── chat_service.go
│   │   ├── similarity_service.go
│   │   ├── template_service.go
│   │   ├── import_service.go
│   │   ├── dashboard_service.go
│   │   ├── parse_service.go
│   │   ├── account_service.go
│   │   └── backup_service.go
│   ├── repository/
│   │   ├── interfaces.go        # 所有 Repository 接口定义
│   │   ├── user_repo.go
│   │   ├── task_repo.go
│   │   ├── upload_repo.go
│   │   ├── evaluation_repo.go
│   │   ├── course_repo.go
│   │   ├── class_repo.go
│   │   ├── notification_repo.go
│   │   ├── chat_repo.go
│   │   ├── similarity_repo.go
│   │   ├── template_repo.go
│   │   ├── import_repo.go
│   │   ├── audit_repo.go
│   │   ├── profile_repo.go
│   │   ├── system_config_repo.go
│   │   └── llm_config_repo.go
│   ├── store/
│   │   ├── sqlite.go            # 连接池初始化、WAL 模式、PRAGMA
│   │   ├── migrate.go           # DDL 迁移（嵌入 SQL）
│   │   └── tx.go                # 事务辅助
│   ├── model/
│   │   ├── user.go
│   │   ├── task.go
│   │   ├── upload.go
│   │   ├── evaluation.go
│   │   ├── course.go
│   │   ├── class.go
│   │   ├── notification.go
│   │   ├── chat.go
│   │   ├── similarity.go
│   │   ├── template.go
│   │   ├── import_job.go
│   │   ├── audit.go
│   │   ├── profile.go
│   │   ├── llm_config.go
│   │   └── system_config.go
│   ├── dto/
│   │   ├── request.go           # 请求 DTO（按路由组分文件）
│   │   └── response.go          # 响应 DTO
│   ├── middleware/
│   │   ├── auth.go              # JWT 验证 + RBAC
│   │   ├── trace.go             # trace_id 生成/传播
│   │   ├── cors.go              # CORS
│   │   ├── ratelimit.go         # 令牌桶限流
│   │   ├── security_headers.go  # 安全响应头
│   │   ├── logger.go            # 请求日志
│   │   └── session_timeout.go   # 会话超时
│   ├── worker/
│   │   ├── pool.go              # goroutine worker pool
│   │   ├── task.go              # 任务定义与状态机
│   │   └── retry.go             # 重试 + 指数退避
│   ├── sse/
│   │   ├── broker.go            # 进程内 pub/sub broker
│   │   └── client.go            # SSE 连接管理
│   ├── llm/
│   │   ├── client.go            # OpenAI 兼容 HTTP 客户端
│   │   ├── stream.go            # SSE 流式响应解析
│   │   ├── circuit.go           # 熔断器
│   │   ├── retry.go             # 重试逻辑
│   │   └── tools.go             # Function Calling 工具定义
│   ├── similarity/
│   │   ├── simhash.go           # SimHash 算法
│   │   └── cosine.go            # 余弦相似度
│   ├── crypto/
│   │   ├── aes.go               # AES-256-GCM 加解密
│   │   └── bcrypt.go            # bcrypt 密码哈希
│   ├── cache/
│   │   └── lru.go               # 进程内 LRU 缓存 + TTL
│   ├── parser/
│   │   ├── docx.go              # DOCX 文本提取
│   │   └── pdf.go               # PDF 文本提取
│   ├── report/
│   │   ├── pdf.go               # PDF 报表生成
│   │   └── excel.go             # Excel 报表生成
│   └── backup/
│       └── backup.go            # SQLite 在线备份
├── migrations/
│   └── *.sql                    # 嵌入式 SQL 迁移文件
├── go.mod
├── go.sum
├── Makefile                     # build, test, lint, cross-compile
└── README.md
```

## Components and Interfaces

### 1. Configuration (internal/config)

```go
package config

// Config holds all application configuration loaded from TES_ env vars / .env file.
type Config struct {
    Env             string // "dev" | "test" | "prod"
    ListenAddr      string // default ":8000"
    DBPath          string // default "./data/app.db"
    UploadRoot      string // default "./data/uploads"
    DistDir         string // default "./dist"
    JWTSecret       string // required, min 32 chars
    JWTAccessTTL    time.Duration
    JWTRefreshTTL   time.Duration
    LLMKeyMaster    string // required, base64-encoded 32 bytes
    CORSOrigins     []string
    MaxUploadSizeMB int
    WorkerCount     int    // default 4
    TaskBufferSize  int    // default 100
    LogLevel        string // "debug" | "info" | "warn" | "error"
    BackupDir       string // default "./data/backups"
    BackupInterval  time.Duration
    BackupRetention time.Duration
}

// Load reads TES_ prefixed env vars, with .env file fallback.
// Returns error if required fields (JWTSecret, LLMKeyMaster) are missing.
func Load() (*Config, error)
```

### 2. Store Layer (internal/store)

```go
package store

// DB wraps *sql.DB with SQLite-specific initialization.
type DB struct {
    Writer *sql.DB // single writer connection (WAL mode)
    Reader *sql.DB // multiple reader connections
}

// Open creates the SQLite database, enables WAL, runs migrations.
func Open(path string) (*DB, error)

// Close gracefully closes both writer and reader pools.
func (db *DB) Close() error

// WithTx executes fn within a transaction on the writer connection.
func (db *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error

// Backup performs an online backup to the specified path.
func (db *DB) Backup(ctx context.Context, destPath string) error
```

### 3. Repository Interfaces (internal/repository)

```go
package repository

// UserRepo defines data access for users.
type UserRepo interface {
    GetByID(ctx context.Context, id int64) (*model.User, error)
    GetByUsername(ctx context.Context, username string) (*model.User, error)
    List(ctx context.Context, params ListParams) ([]model.User, int64, error)
    Create(ctx context.Context, u *model.User) error
    Update(ctx context.Context, u *model.User) error
    Delete(ctx context.Context, id int64) error
    UpdateLoginState(ctx context.Context, id int64, failed int, lockedUntil *time.Time) error
}

// TaskRepo defines data access for training tasks.
type TaskRepo interface {
    GetByID(ctx context.Context, id int64) (*model.TrainingTask, error)
    List(ctx context.Context, params TaskListParams) ([]model.TrainingTask, int64, error)
    Create(ctx context.Context, t *model.TrainingTask) error
    Update(ctx context.Context, t *model.TrainingTask) error
    Delete(ctx context.Context, id int64) error
    UpdateStatus(ctx context.Context, id int64, status string) error
}

// UploadRepo defines data access for file uploads.
type UploadRepo interface {
    GetByID(ctx context.Context, id int64) (*model.Upload, error)
    List(ctx context.Context, params UploadListParams) ([]model.Upload, int64, error)
    Create(ctx context.Context, u *model.Upload) error
    UpdateStatus(ctx context.Context, id int64, status string, parseResult *string) error
    Delete(ctx context.Context, id int64) error
}

// EvaluationRepo defines data access for evaluations.
type EvaluationRepo interface {
    GetByID(ctx context.Context, id int64) (*model.Evaluation, error)
    List(ctx context.Context, params EvalListParams) ([]model.Evaluation, int64, error)
    Create(ctx context.Context, e *model.Evaluation) error
    Update(ctx context.Context, e *model.Evaluation) error
    BatchConfirm(ctx context.Context, ids []int64) error
}

// SimilarityRepo defines data access for similarity records.
type SimilarityRepo interface {
    GetByTaskID(ctx context.Context, taskID int64) ([]model.SimilarityRecord, error)
    Store(ctx context.Context, r *model.SimilarityRecord) error
    GetFingerprints(ctx context.Context, taskID int64) ([]model.Fingerprint, error)
    StoreFingerprint(ctx context.Context, f *model.Fingerprint) error
}

// SystemConfigRepo defines data access for runtime configuration.
type SystemConfigRepo interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    GetAll(ctx context.Context) (map[string]string, error)
}

// ListParams is the common pagination/filter struct.
type ListParams struct {
    Page     int
    PageSize int
    Search   string
    Sort     string
    Order    string // "asc" | "desc"
}
```

### 4. Authentication & Authorization (internal/middleware, internal/crypto)

```go
package crypto

// JWT token operations using crypto/hmac (HS256).

// Claims represents the JWT payload.
type Claims struct {
    Sub  string `json:"sub"`  // user ID as string
    Role string `json:"role"` // "admin" | "teacher" | "student"
    Type string `json:"type"` // "access" | "refresh"
    Iat  int64  `json:"iat"`
    Exp  int64  `json:"exp"`
}

// SignToken creates a signed JWT string.
func SignToken(claims Claims, secret []byte) (string, error)

// VerifyToken validates signature and expiration, returns claims.
func VerifyToken(tokenStr string, secret []byte) (*Claims, error)

// HashPassword hashes a password using bcrypt cost 10.
func HashPassword(plain string) (string, error)

// VerifyPassword checks a plaintext password against a bcrypt hash.
func VerifyPassword(plain, hash string) bool
```

```go
package middleware

// AuthMiddleware validates JWT from Authorization: Bearer header.
// Injects user claims into request context.
func AuthMiddleware(secret []byte) func(http.Handler) http.Handler

// RequireRole returns middleware that checks the user's role.
func RequireRole(roles ...string) func(http.Handler) http.Handler

// SessionTimeout rejects requests if last activity > 30 minutes.
func SessionTimeout(store SessionStore) func(http.Handler) http.Handler
```

### 5. Crypto Module (internal/crypto)

```go
package crypto

// AES-256-GCM encryption for API key storage.
// Format: base64(nonce[12] || ciphertext || tag[16])

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
func Encrypt(plaintext string, masterKey []byte) (string, error)

// Decrypt decrypts a base64-encoded ciphertext.
func Decrypt(ciphertext string, masterKey []byte) (string, error)

// DeriveMasterKey converts a base64 config string to a 32-byte key.
func DeriveMasterKey(b64Key string) ([]byte, error)
```

### 6. Worker Pool (internal/worker)

```go
package worker

// TaskFunc is the function signature for background tasks.
type TaskFunc func(ctx context.Context) error

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
    StatusPending   TaskStatus = "pending"
    StatusRunning   TaskStatus = "running"
    StatusCompleted TaskStatus = "completed"
    StatusFailed    TaskStatus = "failed"
)

// Pool manages a fixed number of worker goroutines.
type Pool struct {
    queue      chan *Task
    workers    int
    maxRetries int
    wg         sync.WaitGroup
}

// NewPool creates a worker pool with the given concurrency and buffer size.
func NewPool(workers, bufferSize, maxRetries int) *Pool

// Submit enqueues a task and returns its ID immediately.
func (p *Pool) Submit(ctx context.Context, name string, fn TaskFunc) (string, error)

// Shutdown signals workers to stop and waits up to timeout for drain.
func (p *Pool) Shutdown(timeout time.Duration) error

// Task represents a unit of background work.
type Task struct {
    ID        string
    Name      string
    Status    TaskStatus
    Attempts  int
    Error     string
    CreatedAt time.Time
    Fn        TaskFunc
}
```

### 7. SSE Broker (internal/sse)

```go
package sse

// EventType identifies the kind of SSE event.
type EventType string

const (
    EventProgress     EventType = "progress"
    EventNotification EventType = "notification"
    EventChatStream   EventType = "chat_stream"
    EventTaskStatus   EventType = "task_status"
)

// Event is a single SSE message.
type Event struct {
    Type   EventType
    UserID int64
    Data   []byte // JSON payload
}

// Broker manages SSE client connections and event distribution.
type Broker struct {
    clients    map[int64]map[*Client]struct{} // userID → set of clients
    register   chan *Client
    unregister chan *Client
    publish    chan Event
}

// NewBroker creates and starts the SSE broker goroutine.
func NewBroker() *Broker

// Subscribe registers a new SSE client for the given user.
func (b *Broker) Subscribe(userID int64, w http.ResponseWriter, r *http.Request) error

// Publish sends an event to all connections for the specified user.
func (b *Broker) Publish(evt Event)
```

### 8. LLM Client (internal/llm)

```go
package llm

// Client calls OpenAI-compatible APIs over HTTPS.
type Client struct {
    httpClient *http.Client
    circuit    *CircuitBreaker
    logger     *slog.Logger
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
    Model       string        `json:"model"`
    Messages    []Message     `json:"messages"`
    Temperature float64       `json:"temperature"`
    MaxTokens   int           `json:"max_tokens,omitempty"`
    Tools       []ToolDef     `json:"tools,omitempty"`
    Stream      bool          `json:"stream"`
}

// ChatResponse represents a non-streaming chat completion response.
type ChatResponse struct {
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

// NewClient creates an LLM client with retry and circuit breaker.
func NewClient(httpClient *http.Client, logger *slog.Logger) *Client

// Complete sends a chat completion request (non-streaming).
func (c *Client) Complete(ctx context.Context, cfg LLMConfig, req ChatRequest) (*ChatResponse, error)

// Stream sends a streaming chat completion request, calling onChunk for each delta.
func (c *Client) Stream(ctx context.Context, cfg LLMConfig, req ChatRequest, onChunk func(string)) error

// Embed generates embedding vectors for the given texts.
func (c *Client) Embed(ctx context.Context, cfg LLMConfig, texts []string) ([][]float64, error)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
    failures    int
    threshold   int           // default 5
    cooldown    time.Duration // default 30s
    state       CircuitState  // closed | open | half-open
    lastFailure time.Time
    mu          sync.Mutex
}

// LLMConfig holds decrypted runtime LLM configuration.
type LLMConfig struct {
    BaseURL        string
    APIKey         string // decrypted at call time
    ChatModel      string
    EmbeddingModel string
}
```

### 9. Similarity Engine (internal/similarity)

```go
package similarity

// SimHash computes a 64-bit SimHash fingerprint for the given text.
func SimHash(text string) uint64

// HammingDistance returns the number of differing bits between two fingerprints.
func HammingDistance(a, b uint64) int

// CosineSimilarity computes cosine similarity between two float64 vectors.
// Returns a value in [0, 1]. Panics if vectors have different lengths.
func CosineSimilarity(a, b []float64) float64

// DetectResult holds the similarity detection output for a submission.
type DetectResult struct {
    Score      float64          // 0-100%
    TopMatches []MatchReference // top 3 most similar
    Flagged    bool             // score > threshold
}

// MatchReference identifies a similar submission.
type MatchReference struct {
    UploadID int64
    Score    float64
}

// Engine orchestrates the two-phase similarity detection.
type Engine struct {
    hamThreshold    int     // max hamming distance for coarse filter
    cosineThreshold float64 // min cosine similarity for flagging
}

// NewEngine creates a similarity engine with configurable thresholds.
func NewEngine(hamThreshold int, cosineThreshold float64) *Engine

// Detect compares a submission against all others in the same task scope.
func (e *Engine) Detect(ctx context.Context, submission Submission, candidates []Submission) (*DetectResult, error)
```

### 10. Cache (internal/cache)

```go
package cache

// LRU is a thread-safe LRU cache with TTL expiration.
type LRU[K comparable, V any] struct {
    capacity int
    ttl      time.Duration
    mu       sync.RWMutex
    items    map[K]*entry[V]
    order    *list.List
}

// New creates an LRU cache with the given capacity and TTL.
func New[K comparable, V any](capacity int, ttl time.Duration) *LRU[K, V]

// Get retrieves a value, returning (value, true) if found and not expired.
func (c *LRU[K, V]) Get(key K) (V, bool)

// Set stores a value with the configured TTL.
func (c *LRU[K, V]) Set(key K, value V)

// Delete removes a key from the cache.
func (c *LRU[K, V]) Delete(key K)

// Cleanup removes all expired entries. Called periodically.
func (c *LRU[K, V]) Cleanup()

// StartCleanup starts a background goroutine that runs Cleanup every interval.
func (c *LRU[K, V]) StartCleanup(interval time.Duration, stop <-chan struct{})
```

### 11. Rate Limiter (internal/middleware)

```go
package middleware

// RateLimiter implements a per-IP sliding window rate limiter.
type RateLimiter struct {
    limit    int           // max requests per window
    window   time.Duration // window size
    mu       sync.Mutex
    counters map[string]*counter
}

// NewRateLimiter creates a rate limiter (default: 10 req/min for auth).
func NewRateLimiter(limit int, window time.Duration) *RateLimiter

// RateLimit returns middleware that enforces the rate limit.
func (rl *RateLimiter) RateLimit() func(http.Handler) http.Handler
```

### 12. Logging (slog integration)

```go
// Logging uses Go stdlib log/slog with JSON handler to stdout.
// Every log entry includes: timestamp, level, trace_id, message, context fields.
// Sensitive fields (password, api_key, secret, token) are automatically redacted.

package middleware

// TraceMiddleware generates a unique trace_id per request (or inherits X-Trace-Id).
// Stores trace_id in context for propagation to services and goroutines.
func TraceMiddleware() func(http.Handler) http.Handler

// RequestLogger logs request entry (method, path) and exit (status, duration_ms).
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler
```

### 13. Static File Server (internal/handler)

```go
package handler

// StaticHandler serves frontend assets from distDir.
// For paths that don't match a static file, falls back to index.html (SPA routing).
func StaticHandler(distDir string) http.Handler
```

### 14. Report Generation (internal/report)

```go
package report

// PDFReport generates a PDF evaluation report with Chinese text support.
type PDFReport struct {
    fontPath string // path to embedded Chinese font
}

// Generate creates a PDF report for the given evaluation data.
func (r *PDFReport) Generate(ctx context.Context, data ReportData) ([]byte, error)

// ExcelReport generates an Excel (.xlsx) statistical report.
type ExcelReport struct{}

// Generate creates an Excel report with statistics (averages, medians, std devs).
func (r *ExcelReport) Generate(ctx context.Context, data StatsData) ([]byte, error)

// ReportData holds evaluation details for PDF generation.
type ReportData struct {
    Student    StudentInfo
    Task       TaskInfo
    Dimensions []DimensionResult
    Comments   string
    Suggestions string
}

// StatsData holds statistical data for Excel generation.
type StatsData struct {
    Filter   ReportFilter
    Students []StudentStats
    Summary  ClassSummary
}
```

### 15. Document Parser (internal/parser)

```go
package parser

// DocxParser extracts text from .docx files using Go stdlib (archive/zip + encoding/xml).
// 解析 word/document.xml 中的 <w:t> 文本节点，按 <w:p> 段落分隔。
type DocxParser struct{}

// Parse extracts structured text content from a .docx file.
func (p *DocxParser) Parse(ctx context.Context, filePath string) (*ParseResult, error)

// PDFParser extracts text from .pdf files using ledongthuc/pdf (pure Go).
// 已验证对中文文字型 PDF 有效。
type PDFParser struct{}

// Parse extracts text content from a PDF file.
// 如果每页提取字符数 < 50，返回 ErrImagePDF 提示调用方走 OCR 路径。
func (p *PDFParser) Parse(ctx context.Context, filePath string) (*ParseResult, error)

// OCRParser delegates image/scanned-PDF recognition to the multimodal LLM API.
// 不使用任何本地 OCR 引擎（无 Tesseract），完全依赖云端多模态 LLM。
// 流程：将页面/图片转为 base64 → 发送给 LLM_Client 的视觉接口 → 返回识别文本。
type OCRParser struct {
    LLMClient *llm.Client
}

// Parse sends base64-encoded image(s) to the multimodal LLM for text recognition.
func (p *OCRParser) Parse(ctx context.Context, filePath string) (*ParseResult, error)

// ParseResult holds the structured output of document parsing.
type ParseResult struct {
    Title      string
    Sections   []Section
    RawText    string
}

// Section represents a hierarchical section in the parsed document.
type Section struct {
    Level    int      // heading level (1-6)
    Title    string
    Content  string
    Figures  []string // figure descriptions
}

// 解析策略（parse_service.go 中编排）：
// 1. .docx → DocxParser（本地，标准库）
// 2. .pdf（文字型）→ PDFParser（本地，ledongthuc/pdf）
// 3. .pdf（提取失败/字符太少）→ OCRParser（云端多模态 LLM）
// 4. .png/.jpg → OCRParser（云端多模态 LLM）
```

### 16. Backup (internal/backup)

```go
package backup

// Manager handles scheduled and on-demand SQLite backups.
type Manager struct {
    db        *store.DB
    dir       string
    interval  time.Duration
    retention time.Duration
    logger    *slog.Logger
}

// NewManager creates a backup manager.
func NewManager(db *store.DB, dir string, interval, retention time.Duration, logger *slog.Logger) *Manager

// RunOnce performs a single backup operation.
func (m *Manager) RunOnce(ctx context.Context) (string, error)

// Start begins the scheduled backup loop.
func (m *Manager) Start(ctx context.Context)

// Cleanup removes backup files older than the retention period.
func (m *Manager) Cleanup() error
```

## Data Models

### SQLite Schema (key tables)

```sql
-- Users
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    real_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL CHECK(role IN ('admin','teacher','student')),
    is_active INTEGER NOT NULL DEFAULT 1,
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until TEXT,  -- RFC3339 timestamp or NULL
    last_login_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Training Tasks
CREATE TABLE training_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','published','closed')),
    teacher_id INTEGER NOT NULL REFERENCES users(id),
    course_id INTEGER REFERENCES courses(id),
    class_id INTEGER REFERENCES classes(id),
    deadline TEXT,
    dimensions TEXT,  -- JSON array of dimension objects
    objective_ratio REAL NOT NULL DEFAULT 0.6,
    subjective_ratio REAL NOT NULL DEFAULT 0.4,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Uploads (student submissions)
CREATE TABLE uploads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    filename TEXT NOT NULL,
    filepath TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploaded'
        CHECK(status IN ('uploaded','parsing','parsed','parse_failed')),
    parse_result TEXT,  -- JSON
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Evaluations
CREATE TABLE evaluations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    upload_id INTEGER NOT NULL REFERENCES uploads(id),
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    student_id INTEGER NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'unsubmitted'
        CHECK(status IN ('unsubmitted','submitted','parsed','scored','confirmed','returned')),
    objective_score REAL,
    subjective_score REAL,
    composite_score REAL,
    dimension_scores TEXT,  -- JSON array
    comments TEXT,
    suggestions TEXT,
    confirmed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Similarity Records
CREATE TABLE similarity_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    upload_id INTEGER NOT NULL REFERENCES uploads(id),
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    compared_upload_id INTEGER NOT NULL REFERENCES uploads(id),
    simhash_distance INTEGER,
    cosine_score REAL,
    flagged INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Fingerprints (for incremental similarity)
CREATE TABLE fingerprints (
    upload_id INTEGER PRIMARY KEY REFERENCES uploads(id),
    task_id INTEGER NOT NULL REFERENCES training_tasks(id),
    simhash INTEGER NOT NULL,       -- uint64 stored as INTEGER
    embedding TEXT NOT NULL,         -- JSON array of float64
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- LLM Configuration
CREATE TABLE llm_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    chat_model TEXT NOT NULL,
    embedding_model TEXT,
    is_active INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- System Configuration (runtime key-value)
CREATE TABLE system_configs (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Audit Log (append-only)
CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id INTEGER,
    detail TEXT,  -- JSON
    ip_address TEXT,
    trace_id TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Notifications
CREATE TABLE notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT,
    is_read INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Chat Sessions & Messages
CREATE TABLE chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id),
    task_id INTEGER REFERENCES training_tasks(id),
    round_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE chat_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER NOT NULL REFERENCES chat_sessions(id),
    role TEXT NOT NULL CHECK(role IN ('user','assistant','system','tool')),
    content TEXT NOT NULL,
    tool_calls TEXT,  -- JSON
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
```

## Error Handling

所有业务错误通过统一的 `AppError` 类型传递，handler 层统一转换为 HTTP 响应：

```go
package apperr

// AppError is the base business error type.
type AppError struct {
    Code       string // machine-readable error code
    Message    string // human-readable message
    HTTPStatus int    // HTTP status code to return
    Detail     any    // optional structured detail (validation errors)
}

func (e *AppError) Error() string { return e.Message }

// Common constructors
func NotFound(msg string) *AppError
func BadRequest(msg string) *AppError
func Unauthorized(msg string) *AppError
func Forbidden(msg string) *AppError
func Conflict(msg string) *AppError
func ValidationFailed(fields []FieldError) *AppError
func Internal(msg string) *AppError

// FieldError represents a single field validation error.
type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

HTTP 错误响应格式（与 Python 后端一致）：

```json
// 单一错误
{"detail": "账号或密码错误"}

// 验证错误 (HTTP 422)
{"detail": [{"field": "username", "message": "不能为空"}]}
```

## Application Lifecycle

```go
func main() {
    // 1. Load config (exit if required params missing)
    cfg := config.MustLoad()

    // 2. Initialize slog JSON logger
    logger := setupLogger(cfg.LogLevel)

    // 3. Open SQLite (WAL mode, run migrations)
    db := store.MustOpen(cfg.DBPath)
    defer db.Close()

    // 4. Initialize infrastructure
    masterKey := crypto.MustDeriveMasterKey(cfg.LLMKeyMaster)
    workerPool := worker.NewPool(cfg.WorkerCount, cfg.TaskBufferSize, 3)
    sseBroker := sse.NewBroker()
    cache := cache.New[string, string](1000, 60*time.Second)
    rateLimiter := middleware.NewRateLimiter(10, time.Minute)

    // 5. Wire repositories → services → handlers
    repos := repository.NewSQLite(db)
    services := service.New(repos, workerPool, sseBroker, masterKey, logger)
    router := handler.NewRouter(cfg, services, sseBroker, rateLimiter, logger)

    // 6. Start background services
    backupMgr := backup.NewManager(db, cfg.BackupDir, cfg.BackupInterval, cfg.BackupRetention, logger)
    go backupMgr.Start(context.Background())
    go cache.StartCleanup(5*time.Minute, make(chan struct{}))

    // 7. Start HTTP server
    srv := &http.Server{Addr: cfg.ListenAddr, Handler: router}
    go srv.ListenAndServe()

    // 8. Wait for shutdown signal (SIGTERM/SIGINT)
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    <-sigCh

    // 9. Graceful shutdown
    logger.Info("shutdown.initiated")
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    srv.Shutdown(ctx)
    workerPool.Shutdown(30 * time.Second)
    logger.Info("shutdown.complete")
}
```

## Business Logic Details

### Scoring Formula

```go
// WeightedScore calculates the weighted dimension score.
// Each dimension has a weight (5-100, sum must equal 100).
// Result = Σ(dimension_score × dimension_weight / 100)
func WeightedScore(dimensions []DimensionScore) float64

// CompositeScore calculates the final composite score.
// composite = objective × objectiveRatio + subjective × subjectiveRatio
// Result rounded to 1 decimal place.
func CompositeScore(objective, subjective, objRatio, subjRatio float64) float64
```

### Task State Machine

```
draft → published → closed
         ↑ (no back-transition allowed)
```

Valid transitions:
- `draft` → `published` (via POST /api/tasks/{id}/publish)
- `published` → `closed` (via POST /api/tasks/{id}/close)

### Grading Workflow States

```
unsubmitted → submitted → parsed → scored → confirmed → returned
```

Each transition has specific preconditions and side effects (notifications, audit logs).

### Chat Limits

- 20 rounds per session
- 500 characters max per user message
- 50 LLM calls per student per day

## Requirements Mapping

| Requirement | Primary Packages |
|---|---|
| Req 1: API 契约兼容性 | handler/, dto/, middleware/ |
| Req 2: 构建与部署 | cmd/server/, Makefile, handler/static.go |
| Req 3: 数据库层 | store/, repository/, migrations/ |
| Req 4: 认证与授权 | crypto/, middleware/auth.go |
| Req 5: 异步任务引擎 | worker/ |
| Req 6: 实时推送 | sse/, handler/sse.go |
| Req 7: LLM 客户端 | llm/ |
| Req 8: 相似度检测 | similarity/ |
| Req 9: 加密与安全 | crypto/, middleware/ |
| Req 10: 配置管理 | config/, repository/system_config_repo.go |
| Req 11: 日志与可观测性 | middleware/trace.go, middleware/logger.go |
| Req 12: 文件上传 | handler/uploads.go, service/upload_service.go |
| Req 13: 业务逻辑保真 | service/ (all business services) |
| Req 14: 健康检查与优雅关闭 | handler/health.go, cmd/server/main.go |
| Req 15: 数据库备份 | backup/ |
| Req 16: 路由完整性 | handler/router.go |
| Req 17: 进程内状态管理 | cache/, sse/broker.go, worker/, middleware/ratelimit.go |
| Req 18: 报表生成 | report/ |
| Req 19: 文档解析集成 | parser/ |

## Testing Strategy

### 测试分层

| 层 | 策略 | 工具 |
|---|---|---|
| 单元测试 | 纯函数逻辑（crypto、similarity、scoring、state machine、cache） | Go testing + rapid (PBT) |
| 服务测试 | Mock repository 接口，验证业务编排 | Go testing + gomock |
| 集成测试 | 真实 SQLite（内存模式），验证 SQL 和事务 | Go testing + testcontainers |
| 契约测试 | 请求/响应 JSON schema 匹配 | Go testing + rapid (PBT) |
| 端到端 | HTTP 测试服务器，验证完整请求链路 | httptest |

### Property-Based Testing

使用 `pgregory.net/rapid` 作为 Go PBT 库，最少 100 次迭代。重点覆盖：
- 加密/解密 round-trip
- JWT sign/verify round-trip
- 密码 hash/verify round-trip
- 评分公式计算
- 状态机转换
- 相似度算法
- 缓存 TTL 行为
- 输入验证
- 限流逻辑

### 覆盖率目标

- 核心算法（crypto、similarity、scoring）：100%
- 业务服务：≥ 80%
- Handler 层：≥ 70%
- 总体：≥ 75%

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: JSON DTO serialization round-trip

*For any* valid domain model object, serializing it to the response JSON DTO and then deserializing back to the domain model SHALL produce an equivalent object with identical field values, types, and null semantics.

**Validates: Requirements 1.2, 1.3**

### Property 2: Pagination envelope correctness

*For any* dataset of N items and valid pagination parameters (page ≥ 1, page_size ≥ 1), the returned envelope SHALL contain: items with length ≤ page_size, total = N, page = requested page, page_size = requested page_size, and items SHALL be the correct slice of the sorted dataset.

**Validates: Requirements 1.6**

### Property 3: SPA fallback routing

*For any* HTTP GET request path that does not match a file in the dist/ directory, the static file server SHALL respond with the contents of index.html and HTTP 200.

**Validates: Requirements 2.7**

### Property 4: Embedding vector storage round-trip

*For any* float64 array (embedding vector), storing it as a JSON TEXT column in SQLite and reading it back SHALL produce a numerically identical array (within float64 precision).

**Validates: Requirements 3.7**

### Property 5: JWT sign/verify round-trip

*For any* valid Claims (user ID, role, type, timestamps), signing with a secret key and then verifying with the same key SHALL return the original claims with all fields preserved.

**Validates: Requirements 4.1, 4.2**

### Property 6: Password hash/verify round-trip

*For any* plaintext password (1-72 bytes), hashing with bcrypt cost 10 and then verifying the same plaintext against the hash SHALL return true, and verifying any different plaintext SHALL return false.

**Validates: Requirements 4.3, 9.3**

### Property 7: RBAC enforcement

*For any* combination of user role and endpoint, the authorization middleware SHALL permit access if and only if the role is in the endpoint's allowed roles set, returning HTTP 403 for denied access and passing through for permitted access.

**Validates: Requirements 4.4, 4.8**

### Property 8: Account lockout threshold

*For any* sequence of N consecutive failed login attempts for a user, the account SHALL be locked if and only if N ≥ 5, and the lock duration SHALL be exactly 15 minutes from the last failed attempt.

**Validates: Requirements 4.5, 13.5**

### Property 9: Session timeout enforcement

*For any* authenticated request where the time since last activity exceeds 30 minutes, the middleware SHALL reject the request with HTTP 401, and for any request within 30 minutes of last activity, the middleware SHALL allow it through.

**Validates: Requirements 4.6**

### Property 10: Worker pool concurrency limit

*For any* set of N submitted tasks where N > worker count, the number of concurrently executing tasks SHALL never exceed the configured worker count.

**Validates: Requirements 5.3**

### Property 11: Task panic recovery

*For any* task that panics during execution, the worker pool SHALL recover without crashing, mark the task as failed with the panic message, and continue processing subsequent tasks.

**Validates: Requirements 5.6**

### Property 12: AES-256-GCM encryption round-trip

*For any* plaintext string and valid 32-byte master key, encrypting and then decrypting SHALL return the original plaintext, and decrypting with a different key SHALL fail with an error.

**Validates: Requirements 9.1, 7.9**

### Property 13: Circuit breaker state transitions

*For any* sequence of LLM call results, the circuit breaker SHALL transition to open state after 5 consecutive failures, reject all requests while open, transition to half-open after 30 seconds cooldown, and return to closed on a successful half-open request.

**Validates: Requirements 7.7**

### Property 14: Path traversal rejection

*For any* filename containing path traversal sequences (../, ..\, absolute paths, null bytes), the upload handler SHALL reject the request with HTTP 422 and never write to a path outside the configured upload directory.

**Validates: Requirements 9.5**

### Property 15: Rate limiting enforcement

*For any* IP address making more than the configured limit of requests within the time window, subsequent requests SHALL be rejected with HTTP 429, and requests within the limit SHALL be allowed.

**Validates: Requirements 9.8**

### Property 16: Secret filtering in logs

*For any* log entry containing fields named password, api_key, secret, or token, the output SHALL replace the field value with "***" and never emit the actual secret value.

**Validates: Requirements 10.6, 11.7**

### Property 17: Structured log format

*For any* log event emitted by the application, the output SHALL be valid JSON containing at minimum: timestamp (RFC3339), level, trace_id, and message fields.

**Validates: Requirements 11.1, 11.2**

### Property 18: Trace ID uniqueness

*For any* set of N concurrent HTTP requests, each request SHALL be assigned a unique trace_id, and the trace_id SHALL propagate to all downstream operations (service calls, background tasks) initiated by that request.

**Validates: Requirements 11.3**

### Property 19: File upload validation

*For any* uploaded file, the system SHALL accept it if and only if: file size is within [1KB, max_upload_size_mb], file extension is in the whitelist (.doc, .docx, .pdf, .png, .jpg, .jpeg), AND the file's magic number matches the declared extension. All other files SHALL be rejected with HTTP 422.

**Validates: Requirements 12.1, 12.2, 12.6**

### Property 20: Upload rejection on closed task

*For any* file upload attempt targeting a training task with status "closed", the system SHALL reject the upload with HTTP 403 regardless of file validity.

**Validates: Requirements 12.7**

### Property 21: Weighted scoring formula

*For any* set of dimension scores and weights where weights sum to 100 and each weight ≥ 5, the weighted score SHALL equal Σ(score_i × weight_i / 100), and the composite score SHALL equal (objective × obj_ratio + subjective × subj_ratio) rounded to 1 decimal place.

**Validates: Requirements 13.1, 13.2**

### Property 22: Task state machine validity

*For any* training task, the only valid state transitions SHALL be draft→published and published→closed. Any other transition attempt SHALL be rejected with an error, and the task's state SHALL remain unchanged.

**Validates: Requirements 13.3**

### Property 23: Grading workflow state machine validity

*For any* evaluation, the only valid state transitions SHALL follow the sequence unsubmitted→submitted→parsed→scored→confirmed→returned. Any out-of-order transition attempt SHALL be rejected with an error.

**Validates: Requirements 13.4**

### Property 24: Chat session limits

*For any* chat session, the system SHALL reject user messages after 20 rounds in the session, reject messages exceeding 500 characters, and reject LLM calls after 50 per student per day.

**Validates: Requirements 13.7**

### Property 25: SimHash similarity property

*For any* two identical texts, their SimHash fingerprints SHALL be equal (hamming distance = 0), and for any two texts differing by a small edit, the hamming distance SHALL be less than or equal to a configurable threshold.

**Validates: Requirements 8.1**

### Property 26: Cosine similarity bounds

*For any* two non-zero float64 vectors of equal length, cosine similarity SHALL be in the range [0, 1], and the cosine similarity of any vector with itself SHALL be exactly 1.0.

**Validates: Requirements 8.2**

### Property 27: Similarity scope isolation

*For any* submission, the similarity engine SHALL only compare it against other submissions within the same task_id scope, never against submissions from different tasks.

**Validates: Requirements 8.4**

### Property 28: LRU cache TTL expiration

*For any* cache entry, after the configured TTL has elapsed, a Get operation SHALL return (zero value, false), and the entry SHALL be removed during the next cleanup cycle.

**Validates: Requirements 17.1, 17.7**

### Property 29: Config environment variable loading

*For any* configuration parameter with the TES_ prefix set as an environment variable, the Config loader SHALL read and parse it correctly, and the same parameter in a .env file SHALL be overridden by the environment variable.

**Validates: Requirements 10.1, 10.2**
