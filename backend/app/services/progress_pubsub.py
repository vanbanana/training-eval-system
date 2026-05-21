"""Task 8.7: 解析进度发布/订阅（使用 Redis Pub/Sub）.

业务侧用 publish_progress(user_id, payload)
WebSocket 端点订阅 progress:{user_id} 频道并把消息转给客户端。
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from redis.asyncio import Redis


def channel(user_id: int) -> str:
    return f"progress:{user_id}"


async def publish_progress(
    redis: Redis,
    *,
    user_id: int,
    upload_id: int,
    status: str,
    progress: int = 0,
    error: str | None = None,
) -> int:
    """发布解析进度；返回订阅者数量."""
    payload = {
        "upload_id": upload_id,
        "status": status,
        "progress": max(0, min(100, progress)),
        "error": error,
    }
    return int(await redis.publish(channel(user_id), json.dumps(payload)))


async def subscribe_for_user(redis: Redis, user_id: int) -> object:
    """返回 PubSub 对象，已订阅 progress:{user_id}.

    使用方式（伪代码）：
        pubsub = await subscribe_for_user(redis, current_user.id)
        async for msg in pubsub.listen():
            if msg["type"] == "message":
                yield msg["data"]
    """
    pubsub = redis.pubsub()
    await pubsub.subscribe(channel(user_id))
    return pubsub
