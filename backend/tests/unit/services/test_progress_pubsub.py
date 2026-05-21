"""Task 8.7 验收：解析进度 Pub/Sub."""

from __future__ import annotations

import asyncio
import json
from collections.abc import AsyncIterator

import pytest
from fakeredis.aioredis import FakeRedis

from app.services.progress_pubsub import (
    channel,
    publish_progress,
    subscribe_for_user,
)


pytestmark = pytest.mark.unit


@pytest.fixture()
async def redis() -> AsyncIterator[FakeRedis]:
    r = FakeRedis(decode_responses=False)
    yield r
    await r.aclose()


async def test_channel_isolated_by_user_id() -> None:
    assert channel(1) != channel(2)


async def test_publish_returns_subscriber_count(redis: FakeRedis) -> None:
    # 没订阅者
    n = await publish_progress(
        redis, user_id=1, upload_id=10, status="parsing", progress=50
    )
    assert n == 0


async def test_subscribed_client_receives_message(redis: FakeRedis) -> None:
    """Given 订阅者；When publish；Then 在 1 秒内收到。"""
    pubsub = await subscribe_for_user(redis, user_id=42)

    async def _listen() -> dict[str, object]:
        async for msg in pubsub.listen():
            if msg["type"] == "message":
                data = msg["data"]
                if isinstance(data, (bytes, bytearray)):
                    data = data.decode("utf-8")
                return json.loads(data)
        raise RuntimeError("no message")

    listener = asyncio.create_task(_listen())
    # 给订阅一点时间生效
    await asyncio.sleep(0.05)

    n = await publish_progress(
        redis, user_id=42, upload_id=99, status="parsed", progress=100
    )
    assert n >= 1

    payload = await asyncio.wait_for(listener, timeout=1.0)
    assert payload["upload_id"] == 99
    assert payload["status"] == "parsed"
    assert payload["progress"] == 100

    await pubsub.unsubscribe()
    await pubsub.aclose()


async def test_progress_clamped_to_0_100(redis: FakeRedis) -> None:
    """超出范围的 progress 被钳制."""
    pubsub = await subscribe_for_user(redis, user_id=1)

    async def _listen() -> dict[str, object]:
        async for msg in pubsub.listen():
            if msg["type"] == "message":
                data = msg["data"]
                if isinstance(data, (bytes, bytearray)):
                    data = data.decode("utf-8")
                return json.loads(data)
        raise RuntimeError("no msg")

    task = asyncio.create_task(_listen())
    await asyncio.sleep(0.05)
    await publish_progress(
        redis, user_id=1, upload_id=1, status="parsing", progress=150
    )
    payload = await asyncio.wait_for(task, timeout=1.0)
    assert payload["progress"] == 100

    await pubsub.unsubscribe()
    await pubsub.aclose()
