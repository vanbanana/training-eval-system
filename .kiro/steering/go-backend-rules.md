---
inclusion: fileMatch
fileMatchPattern: 'go-backend/**/*'
---

# Go 后端开发规则

## 分层（Clean Architecture）

- `cmd/server/` — 入口：配置加载、依赖注入、启动 HTTP server
- `internal/handler/` — HTTP 路由层（chi v5），请求解析 + 响应序列化
- `internal/service/` — 业务编排层，事务边界
- `internal/repository/` — 数据访问接口 + SQLite 实现
- `internal/store/` — database/sql 连接池、迁移、事务管理
- `internal/model/` — 领域模型结构体
- `internal/dto/` — 请求/响应 DTO（与前端 JSON 契约一致）
- `internal/middleware/` — 认证、限流、日志、CORS、trace_id
- `internal/worker/` — goroutine worker pool + channel 任务队列
- `internal/sse/` — Server-Sent Events broker
- `internal/llm/` — OpenAI 兼容 HTTP 客户端 + 熔断 + 重试
- `internal/similarity/` — SimHash + 余弦相似度（纯 Go）
- `internal/crypto/` — AES-256-GCM + bcrypt + JWT
- `internal/cache/` — 进程内 LRU 缓存
- `internal/parser/` — DOCX/PDF 文本提取
- `internal/report/` — PDF/Excel 报表生成
- `internal/backup/` — SQLite 在线备份

**禁止**：handler 直接调 repository、repository 调 service、model 含业务逻辑。

## 强制约束

- 零 CGO：所有依赖必须纯 Go，`CGO_ENABLED=0` 编译通过
- 配置：所有可变参数通过环境变量（`TES_` 前缀）+ .env 文件，禁止硬编码
- 日志：`log/slog` 结构化 JSON 输出到 stdout，所有入口/出口/异常/关键决策点记录，含 `trace_id`
- 错误：业务错误用统一 `AppError` 类型（含 Code + HTTPStatus），禁止裸 `panic`
- 资源管理：所有 IO 用 `defer Close()` 或 context 超时控制
- 并发安全：共享状态用 `sync.Mutex` / `sync.RWMutex` / channel，禁止隐式共享
- 敏感字段：日志自动过滤 password / api_key / secret / token；API Key 用 AES-256-GCM 加密存储
- 命名：Go 标准风格（exported PascalCase, unexported camelCase, 包名小写单词）

## 测试

- 单元测试：纯函数逻辑（crypto、similarity、scoring、state machine）
- 服务测试：Mock repository 接口，验证业务编排
- 集成测试：真实 SQLite（内存模式 `:memory:`）
- 契约测试：请求/响应 JSON schema 匹配前端期望
- Property 测试：`pgregory.net/rapid`，最少 100 次迭代
- 覆盖率：核心算法 100%，其他 ≥ 75%

## 构建

```bash
# 开发构建
go build -o bin/server ./cmd/server

# 生产交叉编译（龙芯 LoongArch）
CGO_ENABLED=0 GOOS=linux GOARCH=loong64 go build -ldflags="-s -w" -o bin/server-loong64 ./cmd/server

# 测试
go test ./... -count=1 -race

# 静态分析
go vet ./...
```

## API 契约

- 所有 API 路由、请求/响应 JSON 格式必须与 Python 版本（`backend/app/schemas/`）完全一致
- 前端不做任何修改，Go 后端是 drop-in replacement
- 参考 Python 版本的 Pydantic schema 作为 JSON 契约的权威定义
