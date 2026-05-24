# Implementation Plan: Go Backend Rewrite

## Overview

将现有 Python FastAPI 后端 1:1 重写为 Go 语言实现。采用 Clean Architecture 分层（handler → service → repository → store），输出单一静态链接 ELF 二进制文件，零 CGO 依赖，可交叉编译至 LoongArch (loong64)。代码位于 `go-backend/` 目录。

## Tasks

- [ ] 1. 项目脚手架与基础设施
  - [ ] 1.1. 初始化 Go 模块与目录结构
    - 创建 `go-backend/` 目录，初始化 `go.mod`（Go 1.25+）
    - 创建完整目录结构：cmd/server/, internal/{config,handler,service,repository,store,model,dto,middleware,worker,sse,llm,similarity,crypto,cache,parser,report,backup}/, migrations/
    - 创建 Makefile（build, test, lint, cross-compile targets）
    - 创建 README.md
    - _Requirements: 2.1, 2.4, 2.5, 2.9_

  - [ ] 1.2. 实现配置加载模块 (internal/config)
    - 实现 Config 结构体，包含所有 TES_ 前缀环境变量
    - 实现 .env 文件加载（godotenv）
    - 实现必填参数校验（JWTSecret, LLMKeyMaster 缺失时 exit）
    - 实现敏感值不输出到日志
    - _Requirements: 10.1, 10.2, 10.3, 10.5, 10.6_

  - [ ] 1.3. 实现 SQLite Store 层 (internal/store)
    - 实现 DB 结构体（Writer/Reader 连接池）
    - 实现 Open()：创建数据库文件、启用 WAL 模式、设置 PRAGMA
    - 实现 WithTx() 事务辅助函数
    - 实现 Backup() 在线备份
    - 实现 Close() 优雅关闭
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.8, 3.9, 3.10_

  - [ ] 1.4. 实现数据库迁移 (internal/store + migrations/)
    - 编写所有 DDL SQL 迁移文件（users, training_tasks, uploads, evaluations, similarity_records, fingerprints, llm_configs, system_configs, audit_logs, notifications, chat_sessions, chat_messages, courses, classes, templates, import_jobs, profiles）
    - 实现 migrate.go：嵌入 SQL 文件，首次启动自动执行
    - 适配 PostgreSQL → SQLite 类型映射
    - _Requirements: 3.5, 3.6, 3.7, 3.8_

- [ ] 2. 认证与加密模块
  - [ ] 2.1. 实现加密模块 (internal/crypto)
    - 实现 AES-256-GCM Encrypt/Decrypt（nonce[12] + ciphertext + tag[16]，base64 编码）
    - 实现 DeriveMasterKey（base64 → 32 字节）
    - 实现 bcrypt HashPassword/VerifyPassword（cost 10）
    - 实现 JWT SignToken/VerifyToken（HMAC-SHA256）
    - _Requirements: 4.1, 4.2, 4.3, 9.1, 9.2, 9.3_

  - [ ] 2.2. 实现认证与授权中间件 (internal/middleware)
    - 实现 AuthMiddleware：从 Authorization: Bearer 提取 JWT，验证签名和过期，注入 Claims 到 context
    - 实现 RequireRole：检查用户角色是否在允许列表中
    - 实现 SessionTimeout：30 分钟无活动拒绝请求
    - 实现账户锁定逻辑（5 次失败 → 15 分钟锁定）
    - _Requirements: 4.4, 4.5, 4.6, 4.7, 4.8_

  - [ ] 2.3. 实现安全中间件 (internal/middleware)
    - 实现 CORS 中间件（可配置 origins）
    - 实现 RateLimiter（令牌桶/滑动窗口，默认 10 req/min for auth）
    - 实现 SecurityHeaders（X-Content-Type-Options, X-Frame-Options, CSP）
    - 实现 TraceMiddleware（生成/继承 trace_id）
    - 实现 RequestLogger（请求入口/出口日志）
    - _Requirements: 9.4, 9.6, 9.7, 9.8, 11.1, 11.2, 11.3, 11.4, 11.7, 11.8_

- [ ] 3. Worker Pool 与 SSE Broker
  - [ ] 3.1. 实现 Worker Pool (internal/worker)
    - 实现 Pool 结构体（buffered channel 队列）
    - 实现 NewPool()、Submit()、Shutdown()
    - 实现 Task 状态机（pending → running → completed/failed）
    - 实现 panic recovery（recover 后标记 failed，继续处理）
    - 实现重试逻辑（指数退避 1s/2s/4s，最多 3 次）
    - 实现优雅关闭（30 秒 drain）
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7, 5.8_

  - [ ] 3.2. 实现 SSE Broker (internal/sse)
    - 实现 Broker 结构体（clients map, register/unregister/publish channels）
    - 实现 NewBroker()：启动事件分发 goroutine
    - 实现 Subscribe()：注册 SSE 客户端连接，支持多连接/用户
    - 实现 Publish()：按 userID 分发事件
    - 实现 keepalive（30 秒心跳）
    - 实现客户端断开清理
    - _Requirements: 6.1, 6.3, 6.4, 6.5, 6.7, 6.8, 6.9_

  - [ ] 3.3. 实现 LRU 缓存 (internal/cache)
    - 实现泛型 LRU[K, V] 结构体（capacity + TTL）
    - 实现 Get/Set/Delete 操作（线程安全）
    - 实现 Cleanup() 定期清理过期条目
    - 实现 StartCleanup() 后台清理 goroutine
    - _Requirements: 17.1, 17.6, 17.7_

- [ ] 4. Checkpoint - 基础设施验证
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 5. 核心业务服务（用户、任务、上传、评价）
  - [ ] 5.1. 实现 Model 层 (internal/model)
    - 定义所有领域模型结构体：User, TrainingTask, Upload, Evaluation, Course, Class, Notification, ChatSession, ChatMessage, SimilarityRecord, Fingerprint, Template, ImportJob, AuditLog, Profile, LLMConfig, SystemConfig
    - _Requirements: 3.2, 3.6_

  - [ ] 5.2. 实现 Repository 接口与 SQLite 实现 (internal/repository)
    - 定义所有 Repository 接口（interfaces.go）
    - 实现 UserRepo（CRUD + 登录状态更新 + 分页查询）
    - 实现 TaskRepo（CRUD + 状态更新 + 分页查询）
    - 实现 UploadRepo（CRUD + 状态更新）
    - 实现 EvaluationRepo（CRUD + 批量确认）
    - 实现 CourseRepo, ClassRepo, NotificationRepo, ChatRepo, SimilarityRepo, TemplateRepo, ImportRepo, AuditRepo, ProfileRepo, SystemConfigRepo, LLMConfigRepo
    - _Requirements: 3.2, 3.5, 3.10, 16.1-16.11_

  - [ ] 5.3. 实现用户服务 (internal/service/user_service.go)
    - 实现用户 CRUD（创建、查询、更新、删除、切换状态）
    - 实现分页/搜索/排序
    - _Requirements: 16.2_

  - [ ] 5.4. 实现认证服务 (internal/service/auth_service.go)
    - 实现 Login（密码验证、锁定检查、token 生成、审计日志）
    - 实现 Logout、Refresh、GetMe
    - 实现失败计数与账户锁定逻辑
    - _Requirements: 4.1, 4.2, 4.3, 4.5, 13.5, 16.1_

  - [ ] 5.5. 实现任务服务 (internal/service/task_service.go)
    - 实现任务 CRUD
    - 实现状态机转换（draft → published → closed）
    - 实现发布/关闭副作用（通知触发）
    - _Requirements: 13.3, 13.6, 16.3_

  - [ ] 5.6. 实现上传服务 (internal/service/upload_service.go)
    - 实现文件上传（大小校验、扩展名白名单、magic number 验证）
    - 实现 SHA-256 校验和计算
    - 实现文件存储（task_id/user_id/ 目录结构）
    - 实现路径遍历防护
    - 实现关闭任务拒绝上传
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.6, 12.7, 9.5, 16.4_

  - [ ] 5.7. 实现评价服务 (internal/service/evaluation_service.go)
    - 实现评价 CRUD + 批量确认
    - 实现加权评分公式（dimension_score × weight / 100）
    - 实现综合评分公式（objective × ratio + subjective × ratio，四舍五入 1 位小数）
    - 实现评分工作流状态机（unsubmitted → submitted → parsed → scored → confirmed → returned）
    - _Requirements: 13.1, 13.2, 13.4, 16.5, 16.6_

- [ ] 6. LLM 客户端与相似度引擎
  - [ ] 6.1. 实现 LLM 客户端 (internal/llm)
    - 实现 Client 结构体（net/http + circuit breaker）
    - 实现 Complete()（非流式请求）
    - 实现 Stream()（SSE 流式响应解析，转发到 SSE Broker）
    - 实现 Embed()（embedding 向量生成）
    - 实现 CircuitBreaker（5 次失败 → open → 30s cooldown → half-open）
    - 实现重试逻辑（指数退避，HTTP 429/5xx/timeout）
    - 实现调用日志（model, endpoint, duration, tokens, status）
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8, 7.9, 7.10_

  - [ ] 6.2. 实现相似度引擎 (internal/similarity)
    - 实现 SimHash()（64-bit 指纹计算）
    - 实现 HammingDistance()（位差异计数）
    - 实现 CosineSimilarity()（float64 向量余弦相似度）
    - 实现 Engine.Detect()（两阶段检测：SimHash 粗筛 → cosine 精排）
    - 实现可配置阈值（从 system_config 加载）
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8_

- [ ] 7. 辅助服务（聊天、通知、报表、画像、导入等）
  - [ ] 7.1. 实现聊天服务 (internal/service/chat_service.go)
    - 实现会话管理（创建、列表、历史）
    - 实现消息发送（500 字符限制、20 轮限制、50 次/天限制）
    - 实现 Function Calling 工具定义与编排
    - 实现上下文注入（任务要求、提交摘要、评分）
    - _Requirements: 13.7, 7.5, 16.9_

  - [ ] 7.2. 实现通知服务 (internal/service/notification_service.go)
    - 实现通知 CRUD + 标记已读
    - 实现通知触发器（任务发布、解析完成、评价完成、退回、弱项更新、截止提醒）
    - 实现 SSE 推送集成
    - _Requirements: 13.6, 6.5, 16.9_

  - [ ] 7.3. 实现报表服务 (internal/report + internal/service/report_service.go)
    - 实现 PDF 报表生成（中文字体支持、评价详情、维度分数）
    - 实现 Excel 报表生成（统计数据：均值、中位数、标准差）
    - 实现报表筛选（时间范围、班级、课程、教师）
    - _Requirements: 18.1, 18.2, 18.3, 18.4, 18.5, 18.6_

  - [ ] 7.4. 实现文档解析服务 (internal/parser + internal/service/parse_service.go)
    - 实现 DocxParser（Go 标准库 archive/zip + encoding/xml 解析 word/document.xml）
    - 实现 PDFParser（ledongthuc/pdf 纯 Go 文本提取，已验证中文有效）
    - 实现 OCRParser（将图片/扫描 PDF 转 base64 发送给多模态 LLM API 识别，不使用本地 OCR）
    - 实现解析策略编排：docx→本地 / pdf文字型→本地 / pdf提取失败→LLM / 图片→LLM
    - 实现解析结果结构化输出（标题层级、段落、图片描述）
    - 实现解析状态更新 + SSE 进度事件
    - 实现超时处理（120s）+ 失败重试
    - _Requirements: 19.1, 19.2, 19.3, 19.4, 19.5, 19.6, 19.7, 19.8, 19.9_

  - [ ] 7.5. 实现剩余服务
    - 实现 ProfileService（画像生成与查询）
    - 实现 DashboardService（按角色聚合统计）
    - 实现 ImportService（批量用户导入）
    - 实现 TemplateService（模板 CRUD）
    - 实现 CourseService / ClassService（课程班级 CRUD + 成员管理）
    - 实现 AuditService（审计日志查询，append-only）
    - 实现 AccountService（个人信息修改、密码修改）
    - 实现 BackupService（在线备份 + 定时备份 + 过期清理）
    - _Requirements: 13.8, 13.9, 15.1, 15.2, 15.3, 15.4, 15.5, 15.6, 16.7, 16.8, 16.9_

- [ ] 8. Checkpoint - 服务层验证
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 9. API Handler 层与路由装配
  - [ ] 9.1. 实现 DTO 层 (internal/dto)
    - 定义所有请求 DTO（按路由组分文件）
    - 定义所有响应 DTO（与 Python 后端 JSON 格式完全一致）
    - 实现分页信封结构（items, total, page, page_size）
    - 实现错误响应结构（detail 字段）
    - _Requirements: 1.2, 1.3, 1.5, 1.6_

  - [ ] 9.2. 实现核心 Handler（auth, users, tasks, uploads, evaluations, grading）
    - 实现 AuthHandler（login, logout, refresh, me）
    - 实现 UsersHandler（CRUD + toggle-status）
    - 实现 TasksHandler（CRUD + publish + close + editing sub-routes）
    - 实现 UploadsHandler（CRUD + retry-parse）
    - 实现 EvaluationsHandler（CRUD + batch-confirm）
    - 实现 GradingHandler（submissions list, detail, update, return）
    - _Requirements: 1.1, 1.4, 16.1, 16.2, 16.3, 16.4, 16.5, 16.6_

  - [ ] 9.3. 实现辅助 Handler（chat, notifications, reports, profiles, similarity, etc.）
    - 实现 ChatHandler, NotificationsHandler, ReportsHandler, ProfilesHandler
    - 实现 SimilarityHandler, TemplatesHandler, ImportsHandler
    - 实现 DashboardHandler, ParseHandler, AccountHandler
    - 实现 CoursesHandler, ClassesHandler, LLMHandler, AuditHandler
    - 实现 AdminHandler（backup endpoint）
    - 实现 DevHandler（_dev endpoints, 仅 dev/test 环境）
    - _Requirements: 1.1, 1.7, 16.8, 16.9, 16.10, 16.11_

  - [ ] 9.4. 实现 SSE Handler 与健康检查
    - 实现 SSE endpoint（GET /api/sse/events）：JWT 认证、多事件类型复用
    - 实现 HealthHandler（GET /healthz）：数据库连通性检查
    - _Requirements: 6.1, 6.2, 6.3, 14.1, 14.2_

  - [ ] 9.5. 实现路由装配与中间件注册 (internal/handler/router.go)
    - 使用 chi v5 注册所有路由组
    - 注册全局中间件链（trace, logger, cors, security_headers, rate_limit）
    - 注册认证中间件（protected routes）
    - 注册角色中间件（admin/teacher/student 权限边界）
    - _Requirements: 1.1, 1.7, 4.4, 4.7, 9.7_

- [ ] 10. 静态文件服务与 SPA Fallback
  - [ ] 10.1. 实现静态文件服务 (internal/handler/static.go)
    - 实现 StaticHandler：从 dist/ 目录服务前端资源
    - 实现 SPA fallback：非静态文件路径返回 index.html
    - 实现 dist/ 目录不存在时的优雅降级
    - _Requirements: 2.6, 2.7_

- [ ] 11. 应用入口与优雅关闭
  - [ ] 11.1. 实现 main.go 入口 (cmd/server/main.go)
    - 实现完整启动流程：config → logger → store → infrastructure → repos → services → router → server
    - 实现 SIGTERM/SIGINT 信号监听
    - 实现优雅关闭（HTTP drain 15s + worker drain 30s）
    - 实现启动日志与关闭日志
    - _Requirements: 14.1, 14.3, 14.4, 14.5, 2.8_

- [ ] 12. 交叉编译与部署验证
  - [ ] 12.1. 实现交叉编译与构建验证
    - 配置 Makefile：CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build
    - 验证产出为单一静态链接 ELF 二进制
    - 验证无 CGO 依赖（ldd 检查）
    - 验证 go vet / staticcheck 无警告
    - _Requirements: 2.1, 2.2, 2.3, 2.5_

- [ ] 13. Checkpoint - 全功能集成验证
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 14. Property-Based Tests（pgregory.net/rapid）
  - [ ]* 14.1. Property Test: JSON DTO round-trip
    - **Property 1: JSON DTO serialization round-trip**
    - 生成任意合法 domain model，序列化为 response DTO JSON 再反序列化回 domain model，验证字段一致
    - **Validates: Requirements 1.2, 1.3**

  - [ ]* 14.2. Property Test: 分页信封正确性
    - **Property 2: Pagination envelope correctness**
    - 生成任意 N 条数据和合法分页参数，验证返回 items 长度 ≤ page_size，total = N，items 为正确切片
    - **Validates: Requirements 1.6**

  - [ ]* 14.3. Property Test: SPA fallback 路由
    - **Property 3: SPA fallback routing**
    - 生成任意非静态文件路径，验证返回 index.html 内容和 HTTP 200
    - **Validates: Requirements 2.7**

  - [ ]* 14.4. Property Test: Embedding 向量存储 round-trip
    - **Property 4: Embedding vector storage round-trip**
    - 生成任意 float64 数组，存入 SQLite TEXT 列再读回，验证数值一致
    - **Validates: Requirements 3.7**

  - [ ]* 14.5. Property Test: JWT sign/verify round-trip
    - **Property 5: JWT sign/verify round-trip**
    - 生成任意合法 Claims，签名后验证返回原始 Claims
    - **Validates: Requirements 4.1, 4.2**

  - [ ]* 14.6. Property Test: 密码 hash/verify round-trip
    - **Property 6: Password hash/verify round-trip**
    - 生成任意 1-72 字节密码，hash 后 verify 同密码返回 true，不同密码返回 false
    - **Validates: Requirements 4.3, 9.3**

  - [ ]* 14.7. Property Test: RBAC 权限执行
    - **Property 7: RBAC enforcement**
    - 生成任意 role + endpoint 组合，验证中间件仅允许 allowed roles 通过
    - **Validates: Requirements 4.4, 4.8**

  - [ ]* 14.8. Property Test: 账户锁定阈值
    - **Property 8: Account lockout threshold**
    - 生成任意 N 次连续失败，验证 N ≥ 5 时锁定，锁定时长 15 分钟
    - **Validates: Requirements 4.5, 13.5**

  - [ ]* 14.9. Property Test: 会话超时
    - **Property 9: Session timeout enforcement**
    - 生成任意时间间隔，验证 > 30min 拒绝，≤ 30min 通过
    - **Validates: Requirements 4.6**

  - [ ]* 14.10. Property Test: Worker Pool 并发限制
    - **Property 10: Worker pool concurrency limit**
    - 提交 N > worker_count 个任务，验证同时执行数不超过 worker_count
    - **Validates: Requirements 5.3**

  - [ ]* 14.11. Property Test: Task panic recovery
    - **Property 11: Task panic recovery**
    - 提交会 panic 的任务，验证 pool 不崩溃、任务标记 failed、后续任务继续执行
    - **Validates: Requirements 5.6**

  - [ ]* 14.12. Property Test: AES-256-GCM round-trip
    - **Property 12: AES-256-GCM encryption round-trip**
    - 生成任意明文和 32 字节 key，加密后解密返回原文；用不同 key 解密失败
    - **Validates: Requirements 9.1, 7.9**

  - [ ]* 14.13. Property Test: 熔断器状态转换
    - **Property 13: Circuit breaker state transitions**
    - 模拟连续失败/成功序列，验证 closed → open（5 failures）→ half-open（30s）→ closed（success）
    - **Validates: Requirements 7.7**

  - [ ]* 14.14. Property Test: 路径遍历拒绝
    - **Property 14: Path traversal rejection**
    - 生成含 ../ 、..\ 、绝对路径、null 字节的文件名，验证全部被拒绝 HTTP 422
    - **Validates: Requirements 9.5**

  - [ ]* 14.15. Property Test: 限流执行
    - **Property 15: Rate limiting enforcement**
    - 生成超过限制的请求序列，验证超限后返回 HTTP 429
    - **Validates: Requirements 9.8**

  - [ ]* 14.16. Property Test: 日志敏感字段过滤
    - **Property 16: Secret filtering in logs**
    - 生成含 password/api_key/secret/token 字段的日志事件，验证输出中值被替换为 "***"
    - **Validates: Requirements 10.6, 11.7**

  - [ ]* 14.17. Property Test: 结构化日志格式
    - **Property 17: Structured log format**
    - 生成任意日志事件，验证输出为合法 JSON 且包含 timestamp/level/trace_id/message
    - **Validates: Requirements 11.1, 11.2**

  - [ ]* 14.18. Property Test: Trace ID 唯一性
    - **Property 18: Trace ID uniqueness**
    - 并发发送 N 个请求，验证每个请求分配唯一 trace_id
    - **Validates: Requirements 11.3**

  - [ ]* 14.19. Property Test: 文件上传校验
    - **Property 19: File upload validation**
    - 生成任意文件（大小、扩展名、magic number 组合），验证仅合法文件被接受
    - **Validates: Requirements 12.1, 12.2, 12.6**

  - [ ]* 14.20. Property Test: 关闭任务拒绝上传
    - **Property 20: Upload rejection on closed task**
    - 对 status="closed" 的任务上传任意合法文件，验证返回 HTTP 403
    - **Validates: Requirements 12.7**

  - [ ]* 14.21. Property Test: 加权评分公式
    - **Property 21: Weighted scoring formula**
    - 生成任意维度分数和权重（sum=100, each≥5），验证公式正确性和四舍五入
    - **Validates: Requirements 13.1, 13.2**

  - [ ]* 14.22. Property Test: 任务状态机合法性
    - **Property 22: Task state machine validity**
    - 生成任意状态转换序列，验证仅 draft→published 和 published→closed 合法
    - **Validates: Requirements 13.3**

  - [ ]* 14.23. Property Test: 评分工作流状态机
    - **Property 23: Grading workflow state machine validity**
    - 生成任意状态转换序列，验证仅按顺序 unsubmitted→submitted→parsed→scored→confirmed→returned 合法
    - **Validates: Requirements 13.4**

  - [ ]* 14.24. Property Test: 聊天会话限制
    - **Property 24: Chat session limits**
    - 验证 20 轮后拒绝、500 字符后拒绝、50 次/天后拒绝
    - **Validates: Requirements 13.7**

  - [ ]* 14.25. Property Test: SimHash 相似度性质
    - **Property 25: SimHash similarity property**
    - 相同文本 hamming distance = 0；小编辑 hamming distance ≤ threshold
    - **Validates: Requirements 8.1**

  - [ ]* 14.26. Property Test: 余弦相似度边界
    - **Property 26: Cosine similarity bounds**
    - 任意非零向量余弦相似度 ∈ [0,1]；自身与自身 = 1.0
    - **Validates: Requirements 8.2**

  - [ ]* 14.27. Property Test: 相似度范围隔离
    - **Property 27: Similarity scope isolation**
    - 验证相似度引擎仅比较同 task_id 内的提交
    - **Validates: Requirements 8.4**

  - [ ]* 14.28. Property Test: LRU 缓存 TTL 过期
    - **Property 28: LRU cache TTL expiration**
    - 设置条目后等待 TTL，验证 Get 返回 false
    - **Validates: Requirements 17.1, 17.7**

  - [ ]* 14.29. Property Test: 配置环境变量加载
    - **Property 29: Config environment variable loading**
    - 设置 TES_ 环境变量和 .env 文件，验证环境变量优先级高于 .env
    - **Validates: Requirements 10.1, 10.2**

- [ ] 15. Final Checkpoint - 全部测试通过
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests use `pgregory.net/rapid` library with minimum 100 iterations
- All 29 correctness properties from design.md are covered in section 14
- Go 代码位于 `go-backend/` 目录，与现有 Python 后端并存
- 交叉编译目标：CGO_ENABLED=0 GOOS=linux GOARCH=loong64

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3"] },
    { "id": 2, "tasks": ["1.4", "2.1"] },
    { "id": 3, "tasks": ["2.2", "2.3", "3.1", "3.2", "3.3"] },
    { "id": 4, "tasks": ["5.1"] },
    { "id": 5, "tasks": ["5.2", "5.4"] },
    { "id": 6, "tasks": ["5.3", "5.5", "5.6", "5.7", "6.1", "6.2"] },
    { "id": 7, "tasks": ["7.1", "7.2", "7.3", "7.4", "7.5"] },
    { "id": 8, "tasks": ["9.1"] },
    { "id": 9, "tasks": ["9.2", "9.3", "9.4"] },
    { "id": 10, "tasks": ["9.5", "10.1", "11.1"] },
    { "id": 11, "tasks": ["12.1"] },
    { "id": 12, "tasks": ["14.1", "14.2", "14.3", "14.4", "14.5", "14.6", "14.7", "14.8", "14.9"] },
    { "id": 13, "tasks": ["14.10", "14.11", "14.12", "14.13", "14.14", "14.15", "14.16", "14.17", "14.18"] },
    { "id": 14, "tasks": ["14.19", "14.20", "14.21", "14.22", "14.23", "14.24", "14.25", "14.26", "14.27", "14.28", "14.29"] }
  ]
}
```
