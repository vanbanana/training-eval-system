# 02 工程原则（编码红线）

本项目作为生产级交付物，严格遵循以下工程原则。这些原则在所有代码、配置、测试中必须被一致执行。**PR 审查时按此 checklist 逐条验证**。

## 1. 配置外置（Configuration as Code）

**原则**：所有可变参数禁止硬编码在源码中，必须通过配置体系注入。

### 三层配置策略

| 层次 | 用途 | 工具 |
|------|------|------|
| 静态默认值 | 类型安全的字段定义、默认值 | `pydantic-settings.BaseSettings` |
| 部署级配置 | 数据库连接、Redis、文件路径、并发数 | `.env` 文件 + 环境变量 |
| 运行时业务配置 | 评分权重比、相似度阈值、限流值、提示词模板 | PostgreSQL `system_config` 表，支持热更新 |

### 禁止清单

- ❌ 在代码中写 `localhost`、`127.0.0.1`、`/data/uploads` 等具体值
- ❌ API Key、密码、密钥写死在源码或 commit 历史
- ❌ 业务阈值（如"相似度 > 0.8"）散落在多处
- ❌ 提示词模板嵌在 Python 字符串里

### 示例

```python
# app/core/config.py
class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_prefix="TES_")
    
    db_url: PostgresDsn
    redis_url: RedisDsn
    upload_root: Path = Path("/data/uploads")
    max_upload_size_mb: int = 50
    similarity_simhash_threshold: int = 6
    similarity_cosine_threshold: float = 0.80
    parse_timeout_seconds: int = 120
    chat_daily_quota: int = 50
    
    @field_validator("max_upload_size_mb")
    @classmethod
    def validate_size(cls, v: int) -> int:
        if not 1 <= v <= 500: raise ValueError("size out of range")
        return v
```

业务规则（评分 α 系数、相似度阈值等）的运行时调整通过 `SystemConfig` 服务读取，TTL 60 秒缓存，管理员后台修改后自动生效。

## 2. 强类型与契约驱动

**原则**：业务边界必须有显式契约，运行时必须类型校验。

- **API 边界**：所有请求/响应通过 Pydantic v2 Schema 定义，禁止 `dict[str, Any]` 直接返回
- **服务层边界**：所有 service 方法签名声明完整类型，return type 不省略
- **数据库边界**：SQLAlchemy 2.0 typed Mapped 风格，与 Pydantic Schema 通过 `model_validate(orm_obj)` 转换
- **LLM 边界**：每个 LLM 调用必须有输入 Pydantic 模型 + 输出 Pydantic 模型，输出失败重试 3 次（容错解析）
- **mypy 配置**：`strict = true`，业务代码 0 警告，第三方边界用 `cast()` 而非 `# type: ignore`

## 3. 日志与可观测性（Observability）

**结构化日志要求**：每条日志必须包含 `timestamp / level / trace_id / span_id / user_id / context_dict`。

### 强制记录点

| 类别 | 何时记录 | 字段 |
|------|---------|------|
| 入口 | 每个 HTTP 请求/Celery 任务开始 | method, path/task_name, payload_size |
| 出口 | 每个 HTTP 请求/Celery 任务结束 | status_code/result, duration_ms |
| 异常 | 所有 catch 块 | exception_type, traceback, ctx |
| 关键业务决策 | 评分计算、状态变更、权限拒绝、相似度判定、LLM 调用 | before/after value, decision_reason |
| 外部调用 | LLM API、OCR、文件存储 | endpoint, duration_ms, tokens, success |

### 实现栈

`structlog` + `python-json-logger`，输出 JSON 行。

### trace_id 透传

HTTP 请求中由中间件生成或从 `X-Trace-Id` 头继承；Celery 任务通过 `headers` 透传；WebSocket 连接握手时分配。

```python
# 入口示例
@router.post("/uploads")
async def create_upload(...):
    log.info("upload.create.start", task_id=task_id, file_size=file.size)
    try:
        result = await upload_service.create(...)
        log.info("upload.create.success", upload_id=result.id, duration_ms=elapsed)
        return result
    except Exception as e:
        log.exception("upload.create.failed", error=str(e), error_type=type(e).__name__)
        raise
```

### 敏感字段过滤

日志中的 `password / api_key / token` 自动用 `***` 替换，由 `structlog` processor 实现。

## 4. 资源管理

**原则**：所有 IO 资源必须通过上下文管理器自动释放，禁止裸 open / connect。

- **数据库会话**：FastAPI Depends 自动管理生命周期，事务用 `async with session.begin():`
- **Redis 连接**：连接池单例，操作通过 `async with pool.acquire():`
- **文件操作**：`async with aiofiles.open(...)` 或同步 `with open(...)`
- **HTTP 客户端**：`httpx.AsyncClient` 全局单例，应用关闭时统一 close
- **分布式锁**：`async with redis_lock("key", ttl=30):`

## 5. 并发安全

**原则**：共享状态必须显式同步，禁止隐式共享。

- **Celery 任务幂等**：所有任务以 `upload_id` 等业务主键作为唯一标识，重复执行不产生重复副作用（数据库唯一约束 + 状态机检查）
- **数据库并发**：用 `SELECT ... FOR UPDATE` 或乐观锁版本号防止 race condition（典型场景：评分修改）
- **Redis 分布式锁**：跨进程互斥操作（如同一 task 的相似度比对）使用 `redlock-py` 或 SET NX EX
- **禁止全局可变变量**：所有"单例"通过 FastAPI 依赖注入或显式 factory，方便测试 mock

## 6. 错误处理与降级

**原则**：失败必须可控，关键路径必须有降级策略。

- 所有 service 方法定义明确的业务异常类（`UploadTooLargeError / TaskClosedError / LLMUnavailableError 等）
- API 层用全局异常处理器映射到 HTTP 状态码 + 标准错误响应
- LLM 调用失败 → 降级手动评分；OCR 失败 → 标记需人工复核；通知推送失败 → 离线时拉取补发
- **熔断器**：连续 5 次外部调用失败触发 30 秒熔断（基于 `purgatory-circuitbreaker` 库）

## 7. 测试驱动与可测试性设计

**原则**：开发完成 = 业务代码 + 测试代码 + 文档，三者缺一不可。

- 业务逻辑写成纯函数（无 IO），方便单元测试
- 所有外部依赖通过 Protocol/ABC 抽象，测试时用 fake 实现替换
- 关键流程在 `dev` 环境暴露 `/api/_dev/*` 调试端点，AI 或测试脚本可独立触发
- 配套 `tes-cli` 命令行工具，无需 UI 即可端到端验证业务

详见 [10 测试与 Dev 端点](10-testing-and-dev-endpoints.md)。

## 8. 单一职责与依赖方向

**原则**：分层架构内依赖必须自上而下，禁止反向依赖。

```
api  →  services  →  repositories  →  models
  ↘                ↗
   schemas (DTO,无依赖)
   
llm/  → 被 services 依赖,自身只依赖 httpx 和 schemas
tasks/ → 被 services 编排, Celery worker 内部独立模块
```

### 禁止清单

- ❌ models 层 import services
- ❌ repositories 层调用 LLM 或 HTTP
- ❌ services 层之间直接互相 import（应通过 service registry 或事件）

## PR 自检清单

提交 PR 前对照本清单逐项确认：

- [ ] 没有硬编码的路径、URL、密钥、阈值
- [ ] 所有公开方法有完整类型注解，`mypy --strict` 通过
- [ ] 入口/出口/异常/关键决策点有结构化日志
- [ ] 所有 IO 资源通过 with/async with 释放
- [ ] 共享状态访问有显式同步
- [ ] 业务异常已定义并被全局处理器捕获
- [ ] 单元测试覆盖核心逻辑，集成测试覆盖主流程
- [ ] 没有反向依赖（如 models import services）
- [ ] 通过 `ruff check` 与 `ruff format --check`
