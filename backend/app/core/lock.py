"""Redis 分布式锁 - 基于 SET NX EX + Lua 脚本原子释放.

使用：
    async with redis_lock("user:1:upload", ttl_seconds=10):
        ... # 临界区

异常：
    LockAcquireTimeoutError - 等待超时
"""

from __future__ import annotations

import asyncio
import secrets
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from redis.asyncio import Redis

from app.core.exceptions import BusinessError
from app.core.redis import get_redis


class LockAcquireTimeoutError(BusinessError):
    error_code = "LOCK_ACQUIRE_TIMEOUT"
    http_status = 409


# Lua: 仅在 token 匹配时 DEL（防止误删别人的锁）
_RELEASE_SCRIPT = """
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
"""


async def _try_acquire(client: Redis, key: str, token: str, ttl: int) -> bool:
    """SET key token NX EX ttl - 返回是否成功获得."""
    return bool(await client.set(name=key, value=token, ex=ttl, nx=True))


async def _release(client: Redis, key: str, token: str) -> int:
    """token 匹配则 DEL；返回删除数量（0 或 1）."""
    return int(await client.eval(_RELEASE_SCRIPT, 1, key, token))  # type: ignore[no-any-return]


@asynccontextmanager
async def redis_lock(
    key: str,
    *,
    ttl_seconds: int = 30,
    blocking: bool = True,
    blocking_timeout: float = 5.0,
    poll_interval: float = 0.05,
    client: Redis | None = None,
) -> AsyncIterator[str]:
    """获取分布式锁；yield 锁 token；退出时原子释放.

    参数：
    - key: 锁名（建议 'res:type:id' 格式）
    - ttl_seconds: 锁自动过期时间（防进程崩溃锁不释放）
    - blocking: 未抢到时是否等待
    - blocking_timeout: 最长等待秒数；超时抛 LockAcquireTimeoutError
    - poll_interval: 轮询间隔
    - client: 注入测试用 redis（默认从全局单例取）
    """
    redis_client = client or get_redis()
    token = secrets.token_hex(16)

    deadline = asyncio.get_event_loop().time() + blocking_timeout
    while True:
        if await _try_acquire(redis_client, key, token, ttl_seconds):
            break
        if not blocking:
            raise LockAcquireTimeoutError(f"锁 {key} 已被占用", field="key")
        if asyncio.get_event_loop().time() >= deadline:
            raise LockAcquireTimeoutError(
                f"等待锁 {key} 超过 {blocking_timeout} 秒", field="key"
            )
        await asyncio.sleep(poll_interval)

    try:
        yield token
    finally:
        # 释放：仅删自己的 token（防止 TTL 已过、别人重新拿到锁后被误删）
        await _release(redis_client, key, token)
