"""Redis 异步连接池单例."""

from __future__ import annotations

from typing import cast

from redis.asyncio import Redis

from app.core.config import get_settings

_redis_singleton: Redis | None = None


def get_redis() -> Redis:
    """惰性初始化全局连接（单例）.

    使用方式（在 FastAPI startup 中预热）：
        from app.core.redis import get_redis
        await get_redis().ping()
    """
    global _redis_singleton
    if _redis_singleton is None:
        settings = get_settings()
        _redis_singleton = cast(
            Redis,
            Redis.from_url(
                settings.redis_url,
                encoding="utf-8",
                decode_responses=True,
                max_connections=20,
            ),
        )
    return _redis_singleton


def set_redis_for_test(client: Redis | None) -> None:
    """测试用：注入 fake redis 客户端，传 None 清空。"""
    global _redis_singleton
    _redis_singleton = client


async def close_redis() -> None:
    """关闭连接池（应用 shutdown 时调用）."""
    global _redis_singleton
    if _redis_singleton is not None:
        await _redis_singleton.aclose()
        _redis_singleton = None
