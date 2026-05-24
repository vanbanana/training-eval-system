---
inclusion: fileMatch
fileMatchPattern: 'backend/**/*'
---

# Python 后端规则（旧版参考，已冻结）

> ⚠️ **注意：Python 后端已冻结，仅作为 Go 重写的业务逻辑参考。不再新增功能。**
> 新后端代码请写在 `go-backend/` 目录，遵循 `go-backend-rules.md`。

## 分层

- `app/api/`：路由 + 依赖注入 + 请求/响应 schema
- `app/services/`：业务编排（Application 层），事务边界
- `app/repositories/`：数据访问（Infrastructure 层）
- `app/models/`：SQLAlchemy ORM 模型
- `app/schemas/`：Pydantic v2 schema
- `app/llm/`：LLM 适配器、Skills、Function Calling 工具
- `app/tasks/`：Celery 任务
- `app/core/`：配置、异常、日志、加密、中间件、锁等横切

**禁止**：API 直接调 Repository、Repository 调 Service、Models 含业务逻辑。

## 强制约束

- 类型：100% 强类型，`mypy --strict` 通过；禁止 `Any` 传业务数据
- 配置：所有可变参数通过 `app.core.config.Settings`（pydantic-settings + `TES_` 前缀），禁止硬编码
- 日志：结构化 JSON（structlog），所有入口/出口/异常/关键决策点记录，含 `trace_id`
- 异常：业务异常继承 `BusinessError`，含 `error_code` + `http_status`，禁止裸 `raise Exception`
- 资源管理：所有 IO / 锁 / 连接用 `async with` 或 `try-finally`
- 并发安全：共享状态用 Redis 锁或 DB 行锁，禁止隐式共享
- 敏感字段：日志自动过滤 password / api_key / secret / token；API Key 用 AES-256-GCM 加密存储

## 测试

- 单元测试（Domain）：100% Mock 外部依赖
- 编排测试（Service）：Mock Repository，验证调用顺序与事务
- 集成测试（Repository / API）：testcontainers 真实 PG/Redis
- 契约测试（API）：输入输出 schema 严格匹配
- 测试数据：用 `tests/factories/*Factory.create(...)`，禁止字面值
- 覆盖率：核心算法 100%，其他 ≥ 70%

## Dev 测试端点

每个核心 Service 必须提供 `/api/_dev/*` 端点（仅 `TES_ENV=dev|test` 启用），返回 JSON 状态/触发动作，方便 AI 自验证。
