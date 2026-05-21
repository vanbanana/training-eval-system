"""Task 3.8 验收：会话超时机制."""

from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator

import pytest
from fakeredis.aioredis import FakeRedis

from app.core.config import get_settings
from app.core.exceptions import AuthenticationError
from app.core.security import create_access_token
from app.core.session_timeout import (
    SessionExpiredError,
    assert_session_alive_from_token,
    check_session_alive,
    invalidate_session,
    session_key,
    touch_session,
)


pytestmark = pytest.mark.unit


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


@pytest.fixture()
async def fake_redis() -> AsyncIterator[FakeRedis]:
    r = FakeRedis(decode_responses=True)
    yield r
    await r.aclose()


class TestTouchSession:
    async def test_touch_creates_key_with_ttl(self, fake_redis: FakeRedis) -> None:
        await touch_session(42, ttl_seconds=60, redis=fake_redis)
        ttl = await fake_redis.ttl(session_key(42))
        assert 0 < ttl <= 60


class TestCheckSession:
    async def test_returns_true_after_touch(self, fake_redis: FakeRedis) -> None:
        await touch_session(1, redis=fake_redis)
        assert await check_session_alive(1, redis=fake_redis) is True

    async def test_returns_false_when_never_touched(
        self, fake_redis: FakeRedis
    ) -> None:
        assert await check_session_alive(99, redis=fake_redis) is False

    async def test_returns_false_after_ttl_expiry(self, fake_redis: FakeRedis) -> None:
        """Given session ttl=1s；When 等 1.5s；Then check 返回 False。"""
        # fakeredis 支持 ttl 模拟，但不会真的过期；用 short ttl + sleep 不会
        # 触发自动删除。改为 manual delete 模拟过期。
        await touch_session(1, ttl_seconds=1, redis=fake_redis)
        # 模拟过期
        await fake_redis.delete(session_key(1))
        assert await check_session_alive(1, redis=fake_redis) is False


class TestInvalidate:
    async def test_logout_removes_session(self, fake_redis: FakeRedis) -> None:
        await touch_session(1, redis=fake_redis)
        assert await check_session_alive(1, redis=fake_redis) is True

        await invalidate_session(1, redis=fake_redis)
        assert await check_session_alive(1, redis=fake_redis) is False


class TestAssertFromToken:
    async def test_active_session_returns_user_id(
        self, fake_redis: FakeRedis
    ) -> None:
        await touch_session(7, redis=fake_redis)
        token = create_access_token(user_id=7, role="student")

        uid = await assert_session_alive_from_token(token, redis=fake_redis)
        assert uid == 7

    async def test_expired_session_raises(self, fake_redis: FakeRedis) -> None:
        token = create_access_token(user_id=8, role="student")
        # 没 touch → 视为过期/登出
        with pytest.raises(SessionExpiredError) as exc:
            await assert_session_alive_from_token(token, redis=fake_redis)
        assert exc.value.error_code == "SESSION_EXPIRED"

    async def test_logout_immediately_invalidates(
        self, fake_redis: FakeRedis
    ) -> None:
        token = create_access_token(user_id=9, role="student")
        await touch_session(9, redis=fake_redis)
        # 立即 logout
        await invalidate_session(9, redis=fake_redis)

        with pytest.raises(SessionExpiredError):
            await assert_session_alive_from_token(token, redis=fake_redis)

    async def test_invalid_token_raises_auth_error(
        self, fake_redis: FakeRedis
    ) -> None:
        with pytest.raises(AuthenticationError):
            await assert_session_alive_from_token("not.a.token", redis=fake_redis)
