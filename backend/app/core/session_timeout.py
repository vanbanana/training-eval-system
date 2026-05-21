"""Session 超时中间件 - 30 分钟无活动自动登出.

策略：
- 每个已认证请求向 Redis 写入 `session:{user_id}:last_active`
- 下次请求若距上次活动 > TTL，返回 401 SESSION_EXPIRED
- logout 端点立即删除该 key
"""

from __future__ import annotations

from time import time
from typing import TYPE_CHECKING

from app.core.exceptions import AuthenticationError
from app.core.redis import get_redis
from app.core.security import decode_token

if TYPE_CHECKING:
    from redis.asyncio import Redis


SESSION_PREFIX = "session:"


class SessionExpiredError(AuthenticationError):
    error_code = "SESSION_EXPIRED"


def session_key(user_id: int) -> str:
    return f"{SESSION_PREFIX}{user_id}:last_active"


async def touch_session(
    user_id: int,
    *,
    ttl_seconds: int = 30 * 60,
    redis: Redis | None = None,
) -> None:
    """更新用户最后活跃时间；TTL 重新计时."""
    cache = redis or get_redis()
    await cache.set(session_key(user_id), str(int(time())), ex=ttl_seconds)


async def check_session_alive(
    user_id: int,
    *,
    redis: Redis | None = None,
) -> bool:
    """检查 session key 是否仍存在（即未过期且未注销）."""
    cache = redis or get_redis()
    return bool(await cache.exists(session_key(user_id)))


async def invalidate_session(
    user_id: int,
    *,
    redis: Redis | None = None,
) -> None:
    """logout 时调用：立即删除 session key."""
    cache = redis or get_redis()
    await cache.delete(session_key(user_id))


async def assert_session_alive_from_token(
    token: str,
    *,
    redis: Redis | None = None,
) -> int:
    """从 access token 解出 user_id 并校验 session 未过期；返回 user_id."""
    payload = decode_token(token)
    try:
        user_id = int(payload["sub"])
    except (KeyError, ValueError, TypeError) as e:
        raise AuthenticationError("token 格式异常") from e

    if not await check_session_alive(user_id, redis=redis):
        raise SessionExpiredError(
            "会话已过期或已登出，请重新登录"
        )
    return user_id
