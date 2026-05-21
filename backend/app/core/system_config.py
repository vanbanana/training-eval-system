"""SystemConfig 服务 - L3 业务级运行时配置.

特性：
- DB 持久化 + Redis 缓存（TTL 60 秒）
- 类型安全 getter（int/float/bool/str/list）
- 类型转换失败 fallback 到 default + 记录 WARNING

使用方式：
    ratio = await SystemConfig.get_float("evaluation.objective_ratio", default=0.6, db=session)
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

from sqlalchemy import select

from app.core.config import get_settings
from app.core.logging import get_logger
from app.core.redis import get_redis
from app.models.system_config import SystemConfig as SysConfModel

if TYPE_CHECKING:
    from redis.asyncio import Redis
    from sqlalchemy.ext.asyncio import AsyncSession


log = get_logger(__name__)
_CACHE_PREFIX = "sysconf:"


def _cache_key(key: str) -> str:
    return f"{_CACHE_PREFIX}{key}"


async def _load_from_cache(redis: Redis, key: str) -> str | None:
    raw = await redis.get(_cache_key(key))
    return raw if isinstance(raw, str) else None


async def _save_to_cache(redis: Redis, key: str, value: Any, ttl: int) -> None:
    if not isinstance(value, str):
        value = json.dumps(value, ensure_ascii=False)
    await redis.set(_cache_key(key), value, ex=ttl)


async def _load_from_db(db: AsyncSession, key: str) -> Any | None:
    stmt = select(SysConfModel).where(SysConfModel.key == key)
    row = (await db.execute(stmt)).scalar_one_or_none()
    return row.value if row else None


class SystemConfig:
    """运行时配置服务.

    所有方法接受 `db: AsyncSession` 显式参数（不依赖全局），
    `redis: Redis | None` 默认从单例取，便于测试注入。
    """

    @staticmethod
    async def get_raw(
        key: str,
        *,
        db: AsyncSession,
        redis: Redis | None = None,
    ) -> Any | None:
        """返回原始值（未类型转换）；不存在返回 None."""
        cache = redis or get_redis()
        try:
            cached = await _load_from_cache(cache, key)
            if cached is not None:
                # JSON 反序列化
                try:
                    return json.loads(cached)
                except (json.JSONDecodeError, TypeError):
                    return cached
        except Exception as e:
            log.warning("system_config.cache_read_failed", key=key, error=str(e))

        # cache miss → DB
        value = await _load_from_db(db, key)
        if value is not None:
            try:
                await _save_to_cache(
                    cache, key, value, ttl=get_settings().system_config_cache_ttl_seconds
                )
            except Exception as e:
                log.warning("system_config.cache_write_failed", key=key, error=str(e))
        return value

    @staticmethod
    async def get_int(
        key: str, default: int, *, db: AsyncSession, redis: Redis | None = None
    ) -> int:
        raw = await SystemConfig.get_raw(key, db=db, redis=redis)
        if raw is None:
            return default
        try:
            return int(raw)
        except (TypeError, ValueError):
            log.warning("system_config.cast_failed", key=key, value=raw, expected="int")
            return default

    @staticmethod
    async def get_float(
        key: str, default: float, *, db: AsyncSession, redis: Redis | None = None
    ) -> float:
        raw = await SystemConfig.get_raw(key, db=db, redis=redis)
        if raw is None:
            return default
        try:
            return float(raw)
        except (TypeError, ValueError):
            log.warning("system_config.cast_failed", key=key, value=raw, expected="float")
            return default

    @staticmethod
    async def get_bool(
        key: str, default: bool, *, db: AsyncSession, redis: Redis | None = None
    ) -> bool:
        raw = await SystemConfig.get_raw(key, db=db, redis=redis)
        if raw is None:
            return default
        if isinstance(raw, bool):
            return raw
        if isinstance(raw, (int, float)):
            return bool(raw)
        if isinstance(raw, str):
            return raw.lower() in {"true", "1", "yes", "on"}
        log.warning("system_config.cast_failed", key=key, value=raw, expected="bool")
        return default

    @staticmethod
    async def get_str(
        key: str, default: str, *, db: AsyncSession, redis: Redis | None = None
    ) -> str:
        raw = await SystemConfig.get_raw(key, db=db, redis=redis)
        if raw is None:
            return default
        return str(raw)

    @staticmethod
    async def get_list(
        key: str,
        default: list[Any],
        *,
        db: AsyncSession,
        redis: Redis | None = None,
    ) -> list[Any]:
        raw = await SystemConfig.get_raw(key, db=db, redis=redis)
        if raw is None:
            return default
        if isinstance(raw, list):
            return raw
        log.warning("system_config.cast_failed", key=key, value=raw, expected="list")
        return default

    @staticmethod
    async def set(
        key: str,
        value: Any,
        *,
        category: str = "general",
        updated_by: int | None = None,
        db: AsyncSession,
        redis: Redis | None = None,
    ) -> None:
        """写入或更新配置；自动 invalidate 缓存."""
        existing = (
            await db.execute(
                select(SysConfModel).where(SysConfModel.key == key)
            )
        ).scalar_one_or_none()
        if existing is None:
            db.add(
                SysConfModel(
                    key=key, value=value, category=category, updated_by=updated_by
                )
            )
        else:
            existing.value = value
            existing.category = category
            existing.updated_by = updated_by
        await db.flush()

        await SystemConfig.invalidate(key, redis=redis)

    @staticmethod
    async def invalidate(key: str, *, redis: Redis | None = None) -> None:
        """删除该 key 的缓存."""
        cache = redis or get_redis()
        try:
            await cache.delete(_cache_key(key))
        except Exception as e:
            log.warning("system_config.invalidate_failed", key=key, error=str(e))
