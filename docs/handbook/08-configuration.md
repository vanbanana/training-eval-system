# 08 配置管理规范

## 配置三层模型

| 层 | 形式 | 生效时机 | 内容举例 |
|----|------|---------|---------|
| L1 默认值 | `Settings` 类字段默认值 | 编译时 | 上传大小默认 50MB |
| L2 部署级 | `.env` 文件 / 环境变量（前缀 `TES_`） | 启动时 | DB_URL, REDIS_URL, LLM_KEY_MASTER, ENV |
| L3 业务级 | `system_config` 表 + Redis 缓存 | 运行时（管理员可改） | 评分客观比 α、相似度阈值、限流 |

读取顺序：L3 → L2 → L1，前者覆盖后者。所有读配置必须经过 `Settings.get()` 或 `SystemConfig.get(key)`，禁止 `os.environ` 直读。

## .env.example 模板

```bash
# === 部署模式 ===
TES_ENV=prod                              # dev | test | prod
TES_DEBUG=false
TES_LOG_LEVEL=INFO

# === 数据库 ===
TES_DB_URL=postgresql+asyncpg://tes:secret@localhost:5432/tes
TES_DB_POOL_SIZE=20
TES_DB_POOL_OVERFLOW=10

# === Redis ===
TES_REDIS_URL=redis://localhost:6379/0
TES_REDIS_MAX_CONNECTIONS=50

# === 文件存储 ===
TES_UPLOAD_ROOT=/data/uploads
TES_BACKUP_ROOT=/data/backups
TES_MAX_UPLOAD_SIZE_MB=50

# === JWT ===
TES_JWT_SECRET=                           # 必填，至少32位
TES_JWT_ALGORITHM=HS256
TES_JWT_ACCESS_TTL_MINUTES=60
TES_JWT_REFRESH_TTL_DAYS=7

# === 加密 ===
TES_LLM_KEY_MASTER=                       # AES-256 主密钥，base64 编码
TES_PASSWORD_BCRYPT_ROUNDS=12

# === Celery ===
TES_CELERY_BROKER_URL=redis://localhost:6379/1
TES_CELERY_RESULT_BACKEND=redis://localhost:6379/2
TES_CELERY_WORKER_CONCURRENCY=4

# === 业务默认值（运行时可被 system_config 覆盖）===
TES_PARSE_TIMEOUT_SECONDS=120
TES_SIMILARITY_HAMMING_THRESHOLD=6
TES_SIMILARITY_COSINE_THRESHOLD=0.80
TES_CHAT_DAILY_QUOTA=50
TES_CHAT_MAX_TOOL_ROUNDS=5
TES_LLM_TIMEOUT_SECONDS=60
TES_LLM_RETRY_MAX=3

# === Dev 调试 ===
TES_DEV_TOKEN=                            # 仅 ENV=dev 启用调试端点时必填
```

## system_config 表设计

```sql
CREATE TABLE system_config (
  key         VARCHAR(64) PRIMARY KEY,
  value       JSONB NOT NULL,
  category    VARCHAR(32) NOT NULL,       -- evaluation | similarity | chat | llm | ui
  description TEXT,
  updated_by  BIGINT REFERENCES users(id),
  updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);
```

业务代码读取：

```python
ratio = await SystemConfig.get_float("evaluation.objective_ratio", default=0.6)
```

`SystemConfig.get` 内部走 Redis 缓存（TTL 60 秒），管理员后台修改时主动失效。

## Settings 类示例

```python
# app/core/config.py
from pydantic import PostgresDsn, RedisDsn, Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict
from pathlib import Path

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_prefix="TES_",
        case_sensitive=False,
    )
    
    env: str = Field(default="prod", pattern="^(dev|test|prod)$")
    debug: bool = False
    log_level: str = "INFO"
    
    db_url: PostgresDsn
    db_pool_size: int = Field(default=20, ge=1, le=200)
    db_pool_overflow: int = Field(default=10, ge=0)
    
    redis_url: RedisDsn
    redis_max_connections: int = 50
    
    upload_root: Path = Path("/data/uploads")
    backup_root: Path = Path("/data/backups")
    max_upload_size_mb: int = Field(default=50, ge=1, le=500)
    
    jwt_secret: str = Field(min_length=32)
    jwt_algorithm: str = "HS256"
    jwt_access_ttl_minutes: int = 60
    jwt_refresh_ttl_days: int = 7
    
    llm_key_master: str = Field(min_length=44)  # base64(32 bytes)
    password_bcrypt_rounds: int = Field(default=12, ge=10)
    
    celery_broker_url: RedisDsn
    celery_result_backend: RedisDsn
    celery_worker_concurrency: int = 4
    
    parse_timeout_seconds: int = 120
    similarity_hamming_threshold: int = 6
    similarity_cosine_threshold: float = Field(default=0.80, ge=0.0, le=1.0)
    chat_daily_quota: int = 50
    chat_max_tool_rounds: int = 5
    llm_timeout_seconds: int = 60
    llm_retry_max: int = 3
    
    dev_token: str = ""
    
    @field_validator("dev_token")
    @classmethod
    def check_dev_token(cls, v: str, info):
        if info.data.get("env") == "dev" and not v:
            raise ValueError("dev_token required when env=dev")
        return v

settings = Settings()
```

## system_config 默认 key 清单

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `evaluation.objective_ratio` | float | 0.6 | 客观评分权重 α |
| `evaluation.dimension_min_count` | int | 2 | 维度最少个数 |
| `evaluation.dimension_max_count` | int | 10 | 维度最多个数 |
| `evaluation.dimension_min_weight` | int | 5 | 单维度最小权重 |
| `similarity.hamming_threshold` | int | 6 | SimHash 距离阈值 |
| `similarity.cosine_threshold` | float | 0.80 | 余弦相似度警告阈值 |
| `chat.daily_quota` | int | 50 | 学生每日 AI 问答次数 |
| `chat.max_tool_rounds` | int | 5 | 单次问答最多工具调用 |
| `chat.disabled_tools` | list[str] | [] | 临时禁用的工具名单 |
| `parse.timeout_seconds` | int | 120 | 解析超时 |
| `notification.deadline_remind_hours` | int | 24 | 截止前提醒时长 |
| `audit.retention_months` | int | 12 | 审计日志保留期 |

## 配置变更流程

1. **L1 默认值**：改代码 → 走 PR 流程 → 部署
2. **L2 部署级**：改 `.env` → 重启服务（systemd reload）
3. **L3 业务级**：管理员后台修改 → 60 秒内自动生效（无需重启）

## 安全规范

- API Key、密码、密钥等敏感字段：
  - 数据库存储用 AES-256-GCM 加密（主密钥从 `TES_LLM_KEY_MASTER` 读取）
  - 日志中自动用 `***` 替换
  - 界面回显仅显示掩码
- `.env` 文件：
  - 不得 commit 到 git（`.gitignore` 已包含）
  - 部署服务器权限设为 600
  - 主密钥旋转流程见 `docs/operations.md`
