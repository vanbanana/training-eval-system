"""应用配置 - L2 部署级（环境变量 / .env 文件）."""

from __future__ import annotations

from functools import lru_cache
from typing import Literal

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """配置全部外置；通过 TES_ 前缀环境变量或 .env 注入."""

    model_config = SettingsConfigDict(
        env_prefix="TES_",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    # 运行环境
    env: Literal["dev", "test", "prod"] = "dev"
    debug: bool = False

    # 数据库
    db_url: str = Field(
        default="sqlite+aiosqlite:///./tes_dev.db",
        description="SQLAlchemy async URL，dev 默认 sqlite，prod 用 postgresql+asyncpg",
    )

    # Redis（dev 可选）
    redis_url: str = "redis://localhost:6379/0"

    # JWT
    jwt_secret: str = Field(
        default="dev-secret-key-please-change-in-production-32chars",
        min_length=32,
        description="JWT 签名密钥，长度 ≥ 32",
    )
    jwt_algorithm: str = "HS256"
    jwt_access_ttl_minutes: int = 60
    jwt_refresh_ttl_days: int = 7

    # LLM API Key 主密钥（base64 编码的 32 字节）
    llm_key_master: str = Field(
        default="ZGV2LWxsbS1tYXN0ZXIta2V5LWZvci1kZXZlbG9wbWVudC0zMmJ5",
        min_length=32,
        description="AES-256 主密钥（base64），用于加密存储 LLM API Key",
    )

    # 上传
    upload_root: str = "./data/uploads"
    max_upload_size_mb: int = Field(default=50, ge=1, le=500)

    # system_config 缓存 TTL（秒）
    system_config_cache_ttl_seconds: int = Field(default=60, ge=1, le=3600)

    # CORS
    cors_origins: list[str] = ["http://localhost:5173", "http://localhost:3000"]

    # Dev / 测试调试端点 token（仅 env=dev|test 时生效）
    dev_token: str = "dev-token"

    @field_validator("env")
    @classmethod
    def _validate_env(cls, v: str) -> str:
        if v not in {"dev", "test", "prod"}:
            raise ValueError(f"env must be one of dev/test/prod, got {v}")
        return v


@lru_cache
def get_settings() -> Settings:
    """单例获取（lru_cache 保证测试可 override）."""
    return Settings()
