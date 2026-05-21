"""Task 1.7 验收：Redis 分布式锁（用 fakeredis 模拟）."""

from __future__ import annotations

import asyncio

import pytest
from fakeredis.aioredis import FakeRedis

from app.core.lock import LockAcquireTimeoutError, redis_lock


@pytest.fixture()
async def fake_redis() -> FakeRedis:
    client = FakeRedis(decode_responses=True)
    yield client
    await client.aclose()


class TestLockHappyPath:
    async def test_lock_serializes_concurrent_holders(self, fake_redis: FakeRedis) -> None:
        """Given 协程 A、B 抢同一锁；When A 持锁 0.2s；
        Then B 等到 A 释放后才能进入；最终都不抛异常."""
        order: list[str] = []

        async def hold(name: str, hold_time: float) -> None:
            async with redis_lock(
                "k1",
                ttl_seconds=10,
                blocking_timeout=5.0,
                poll_interval=0.01,
                client=fake_redis,
            ):
                order.append(f"{name}-enter")
                await asyncio.sleep(hold_time)
                order.append(f"{name}-leave")

        await asyncio.gather(hold("A", 0.2), hold("B", 0.05))

        # 不论 A B 谁先拿到，必须是序列化执行
        assert len(order) == 4
        # 第一个 enter 必须先于自己的 leave，再于第二个 enter
        assert order[0].endswith("enter")
        assert order[1].endswith("leave")
        assert order[2].endswith("enter")
        assert order[3].endswith("leave")

    async def test_lock_releases_on_normal_exit(self, fake_redis: FakeRedis) -> None:
        async with redis_lock("k", ttl_seconds=10, client=fake_redis):
            assert await fake_redis.get("k") is not None
        # 退出后键被删除
        assert await fake_redis.get("k") is None

    async def test_lock_releases_on_exception(self, fake_redis: FakeRedis) -> None:
        with pytest.raises(RuntimeError, match="bizz"):
            async with redis_lock("k", ttl_seconds=10, client=fake_redis):
                raise RuntimeError("bizz")
        assert await fake_redis.get("k") is None


class TestLockTimeout:
    async def test_blocking_timeout_raises(self, fake_redis: FakeRedis) -> None:
        """Given A 持锁 1s，B 等 0.1s；When B 等不到；Then 抛 LockAcquireTimeoutError."""

        async def hold_long() -> None:
            async with redis_lock("k", ttl_seconds=10, client=fake_redis):
                await asyncio.sleep(1.0)

        async def try_short() -> None:
            await asyncio.sleep(0.05)
            with pytest.raises(LockAcquireTimeoutError) as exc_info:
                async with redis_lock(
                    "k",
                    ttl_seconds=10,
                    blocking=True,
                    blocking_timeout=0.1,
                    poll_interval=0.01,
                    client=fake_redis,
                ):
                    pass
            assert exc_info.value.error_code == "LOCK_ACQUIRE_TIMEOUT"

        await asyncio.gather(hold_long(), try_short())

    async def test_non_blocking_raises_immediately(self, fake_redis: FakeRedis) -> None:
        """Given 锁已占用、blocking=False；When 二次获取；Then 立即抛。"""
        async with redis_lock("k", ttl_seconds=10, client=fake_redis):
            with pytest.raises(LockAcquireTimeoutError):
                async with redis_lock(
                    "k", ttl_seconds=10, blocking=False, client=fake_redis
                ):
                    pytest.fail("should not enter")


class TestLockSafety:
    async def test_token_isolation_no_cross_delete(self, fake_redis: FakeRedis) -> None:
        """Given A 持锁，TTL=1s 已过；When B 获得新锁后 A 退出；
        Then A 的 finally **不应**误删 B 的锁。"""
        # 直接构造 race 场景
        from app.core.lock import _release, _try_acquire

        a_token = "token-A"
        b_token = "token-B"

        # A 拿锁
        assert await _try_acquire(fake_redis, "k", a_token, ttl=10)

        # 模拟 A 的 TTL 过期：手动删 key
        await fake_redis.delete("k")

        # B 拿到新锁
        assert await _try_acquire(fake_redis, "k", b_token, ttl=10)

        # A 退出 → 释放（应该 NOT 删 B 的锁）
        deleted = await _release(fake_redis, "k", a_token)
        assert deleted == 0

        # B 的锁仍然存在
        assert await fake_redis.get("k") == b_token


class TestLockBoundary:
    async def test_lock_after_ttl_expiry_can_be_acquired(self, fake_redis: FakeRedis) -> None:
        """Given 锁 TTL=1s 后过期；When 新获取；Then 立即成功。"""
        from app.core.lock import _try_acquire

        assert await _try_acquire(fake_redis, "k", "tA", ttl=1)
        # 模拟过期
        await fake_redis.delete("k")
        assert await _try_acquire(fake_redis, "k", "tB", ttl=1)
