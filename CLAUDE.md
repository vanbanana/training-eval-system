# 智能实训评价管理系统 — 铁律

> Go 后端唯一。Python `backend/` 已废弃，所有后端工作只在 `go-backend/` 进行。

## 技术栈

| 层 | 选型 |
|----|------|
| 语言 | Go 1.25+, CGO_ENABLED=0 |
| HTTP | go-chi/chi v5 |
| 数据库 | modernc.org/sqlite (纯 Go, 零 CGO) |
| 认证 | HMAC-SHA256 JWT + bcrypt |
| 加密 | AES-256-GCM |
| 异步 | goroutine worker pool + buffered channel |
| 实时 | SSE (net/http 原生) |
| LLM | net/http 直调 OpenAI 兼容 API |
| 日志 | log/slog (JSON → stdout) |
| 报表 | excelize + go-pdf/fpdf |
| 测试 | 标准 testing + rapid (property-based) |

## 架构分层

```
cmd/server/main.go          # 入口：组装依赖 → 启动 HTTP
internal/
  config/                    # TES_ 环境变量配置 (.env 兜底)
  store/                     # SQLite 连接池 (WAL, 分离读写) + 嵌入式迁移
  model/                     # 领域模型 (纯 struct, 无行为)
  repository/                # 数据访问接口 + SQL 实现
  service/                   # 业务编排 (无 HTTP 感知)
  handler/                   # HTTP 路由 → 参数解析 → 调用 service → JSON 响应
  dto/                       # 请求/响应 DTO
  middleware/                 # auth(JWT+角色), cors, trace, ratelimit, logger, security_headers
  worker/                    # goroutine pool (异步任务)
  sse/                       # SSE broker (实时推送)
  llm/                       # LLM HTTP client + 熔断
  similarity/                # SimHash + cosine 查重
  crypto/                    # AES-256-GCM, bcrypt, JWT
  apperr/                    # 统一应用错误
  cache/                     # LRU + TTL 内存缓存
  parser/                    # docx, pdf, OCR 文档解析
  pipeline/                  # 评阅流水线 (解析→评分→验证→查重)
  report/                    # PDF/Excel 报告生成
  backup/                    # SQLite 在线备份 (VACUUM INTO)
```

## 铁律

### 1. 依赖注入 — 显式构造函数模式

- 每个组件通过 `NewXxx(dep1, dep2)` 构造函数创建
- 依赖通过 interface 传递（定义在 `repository/interfaces.go` 或各包内）
- `cmd/server/main.go` 是唯一的组装点
- **禁止**全局变量、包级 `init()` 做业务初始化

### 2. Context 贯穿

- 所有 repository/service 方法第一个参数是 `context.Context`
- Handler 从 `r.Context()` 获取，向下传递
- 绝不使用 `context.Background()` 处理用户请求

### 3. 错误处理

- Repository: 用 `fmt.Errorf("repo: xxx: %w", err)` 包装
- Service: 用 `fmt.Errorf("service: xxx: %w", err)` 包装
- Handler: 调用 `Error(w, status, message)` 返回 JSON
- 应用级错误类型定义在 `internal/apperr/`

### 4. SQLite 特性

- WAL 模式 (`PRAGMA journal_mode=WAL`)
- 读写分离: `db.Writer` (1 conn) 和 `db.Reader` (4 conns)
- 外键强制 (`PRAGMA foreign_keys=ON`)
- 迁移用嵌入式 SQL (`internal/store/migrations/*.sql`)，启动时自动执行
- 备份用 `VACUUM INTO`，在线执行不阻塞
- 测试用 `:memory:` (shared cache)

### 5. API 约定

- `/healthz` — 健康检查 (公开)
- `/api/auth/*` — 认证 (登录/刷新)
- `/api/*` — 业务接口 (需 JWT)
- 角色: `admin` / `teacher` / `student`
- JSON 请求/响应，`Content-Type: application/json`
- 分页参数: `page`, `page_size`, `search`, `sort_by`, `sort_dir`
- 错误响应格式: `{"detail": "error message"}`

### 6. 认证

- JWT 验证中间件注入 `claims` 到 context
- `GetClaims(ctx)` 提取 claims
- `RequireRole(roles...)` 检查角色
- 支持 token 刷新 (access + refresh 双 token)

### 7. 日志

- 只用 `log/slog`，JSON 格式输出到 stdout
- 级别: `debug` / `info` / `warn` / `error`
- 关键操作必须打日志（登录、评估、导入等）

### 8. 测试

- 单测: `go test ./... -count=1 -race`
- Property-based: `pgregory.net/rapid`（用于 cache, config, crypto, similarity 等）
- 集成测试: `testutil.SetupTestApp(t)` 创建完整内存应用
- 测试用 `:memory:` SQLite，每次测试独立

### 9. 代码风格

- `gofmt -s -w .` 统一格式化
- `go vet ./...` 零警告
- 注释用英文描述意图（包级必须写 `// Package xxx ...`）
- DTO json tag 用小写 snake_case
- Go struct 字段用驼峰

### 10. 配置

- 全部通过 `TES_` 前缀环境变量，`.env` 文件兜底
- 必填: `TES_JWT_SECRET` (≥32 字符), `TES_LLM_KEY_MASTER` (base64, 32 字节)
- 默认值合理，dev 环境可开箱即用
- `TES_ENV` 必须为 `dev` / `test` / `prod` 之一

### 11. 构建部署

- 目标: 龙芯 LoongArch + 银河麒麟 V10/V11
- 交叉编译: `CGO_ENABLED=0 GOOS=linux GOARCH=loong64`
- 输出: 单一静态 ELF 二进制 + `/dist` 前端静态文件
- `make build` 本地构建, `make cross-compile` 交叉编译

## 常用命令

```bash
cd go-backend

# 构建
make build              # 本地
make cross-compile      # LoongArch

# 运行
make run                # go run ./cmd/server

# 测试
make test               # go test ./... -count=1 -race
make test-cover         # + 覆盖率报告

# 代码质量
make lint               # go vet + staticcheck
make fmt                # gofmt -s -w .

# 验证纯静态链接
make verify-static
```

## 禁止事项

- ❌ 修改 Python `backend/` — 已废弃
- ❌ 引入 CGO 依赖 — 必须保持 `CGO_ENABLED=0`
- ❌ 使用 ORM — 直接用 SQL
- ❌ 引入第三方 Web 框架 — chi v5 足够
- ❌ 在 handler 里写业务逻辑 — 到 service 层
- ❌ 在 service 里操作 HTTP — 只处理业务
- ❌ 用 `context.Background()` 处理请求
- ❌ 全局变量存业务状态
