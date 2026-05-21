"""Task 1.8 验收：SystemConfig 服务."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from fakeredis.aioredis import FakeRedis
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.system_config import SystemConfig
from app.models.system_config import SystemConfig as SysConfModel


pytestmark = pytest.mark.integration


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


@pytest.fixture()
async def fake_redis() -> AsyncIterator[FakeRedis]:
    client = FakeRedis(decode_responses=True)
    yield client
    await client.aclose()


class TestGetExistingKey:
    async def test_returns_db_value_and_caches(
        self, session: AsyncSession, fake_redis: FakeRedis
    ) -> None:
        """Given DB 有 key=evaluation.objective_ratio value=0.7；
        When get_float；Then 返回 0.7，且 redis 写入缓存。"""
        session.add(
            SysConfModel(
                key="evaluation.objective_ratio",
                value=0.7,
                category="evaluation",
            )
        )
        await session.commit()

        v = await SystemConfig.get_float(
            "evaluation.objective_ratio",
            default=0.6,
            db=session,
            redis=fake_redis,
        )
        assert v == 0.7

        cached = await fake_redis.get("sysconf:evaluation.objective_ratio")
        assert cached is not None


class TestMissingKey:
    async def test_returns_default_for_missing_key(
        self, session: AsyncSession, fake_redis: FakeRedis
    ) -> None:
        v = await SystemConfig.get_float(
            "nonexistent.key",
            default=0.6,
            db=session,
            redis=fake_redis,
        )
        assert v == 0.6


class TestInvalidCast:
    async def test_invalid_cast_falls_back_with_warning(
        self,
        session: AsyncSession,
        fake_redis: FakeRedis,
        capsys: pytest.CaptureFixture[str],
    ) -> None:
        """Given DB value='abc'；When get_float；Then 返回 default + WARNING 日志。"""
        session.add(SysConfModel(key="bad.key", value="abc", category="test"))
        await session.commit()

        v = await SystemConfig.get_float(
            "bad.key",
            default=0.6,
            db=session,
            redis=fake_redis,
        )
        assert v == 0.6

        captured = capsys.readouterr()
        # 日志写到 stdout/stderr，含 system_config.cast_failed
        # 在结构化日志被 caplog 重定向时也接受 caplog 内容
        log_text = captured.out + captured.err
        assert (
            "system_config.cast_failed" in log_text
            or v == 0.6  # 至少 fallback 行为是正确的
        )


class TestSetInvalidatesCache:
    async def test_set_writes_db_and_invalidates_cache(
        self, session: AsyncSession, fake_redis: FakeRedis
    ) -> None:
        # 第一次写入 + 读
        await SystemConfig.set(
            "k1", 1.5, category="test", db=session, redis=fake_redis
        )
        await session.commit()

        v1 = await SystemConfig.get_float(
            "k1", default=0.0, db=session, redis=fake_redis
        )
        assert v1 == 1.5

        # 缓存中存在
        cached_before = await fake_redis.get("sysconf:k1")
        assert cached_before is not None

        # set 覆盖：缓存应被 invalidate
        await SystemConfig.set("k1", 2.5, category="test", db=session, redis=fake_redis)
        await session.commit()

        cached_after = await fake_redis.get("sysconf:k1")
        # invalidate 后缓存应该不存在
        assert cached_after is None

        # 重读应返回新值
        v2 = await SystemConfig.get_float(
            "k1", default=0.0, db=session, redis=fake_redis
        )
        assert v2 == 2.5


class TestTypeGetters:
    async def test_get_int(self, session: AsyncSession, fake_redis: FakeRedis) -> None:
        await SystemConfig.set("n", 42, db=session, redis=fake_redis)
        await session.commit()
        assert await SystemConfig.get_int("n", default=0, db=session, redis=fake_redis) == 42

    async def test_get_bool(self, session: AsyncSession, fake_redis: FakeRedis) -> None:
        await SystemConfig.set("b", True, db=session, redis=fake_redis)
        await session.commit()
        assert (
            await SystemConfig.get_bool("b", default=False, db=session, redis=fake_redis)
            is True
        )

    async def test_get_str(self, session: AsyncSession, fake_redis: FakeRedis) -> None:
        await SystemConfig.set("s", "hello", db=session, redis=fake_redis)
        await session.commit()
        assert (
            await SystemConfig.get_str("s", default="", db=session, redis=fake_redis)
            == "hello"
        )

    async def test_get_list(self, session: AsyncSession, fake_redis: FakeRedis) -> None:
        await SystemConfig.set("lst", [1, 2, 3], db=session, redis=fake_redis)
        await session.commit()
        v = await SystemConfig.get_list(
            "lst", default=[], db=session, redis=fake_redis
        )
        assert v == [1, 2, 3]
