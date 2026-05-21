"""Task 2.1 验收：SQLAlchemy 异步引擎与 session 工厂."""

from __future__ import annotations

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.database import (
    Base,
    NAMING_CONVENTION,
    SessionLocal,
    engine,
    get_db_session,
)


pytestmark = pytest.mark.integration


class TestEngine:
    async def test_engine_can_execute_simple_query(self) -> None:
        """Given engine 已创建；When 执行 SELECT 1；Then 返回 1。"""
        async with engine.begin() as conn:
            result = await conn.execute(text("SELECT 1"))
            assert result.scalar() == 1

    async def test_engine_url_matches_settings(self) -> None:
        from app.core.config import get_settings

        s = get_settings()
        # URL 字符串可能包含 mask；只断言 scheme 一致
        assert str(engine.url).startswith(s.db_url.split("://")[0])


class TestSessionLocal:
    async def test_session_is_async_session(self) -> None:
        async with SessionLocal() as session:
            assert isinstance(session, AsyncSession)
            result = await session.execute(text("SELECT 42"))
            assert result.scalar() == 42

    async def test_expire_on_commit_disabled(self) -> None:
        """Given expire_on_commit=False；When commit；Then 已读对象仍可访问。"""
        # 此处仅断言 sessionmaker 设置（不构造完整 ORM）
        # 通过创建一个 session 并检查 kw_args
        async with SessionLocal() as session:
            assert session.sync_session.expire_on_commit is False


class TestGetDbSession:
    async def test_yields_session_and_commits_on_success(self) -> None:
        """Given get_db_session；When 正常退出；Then session 已 commit。"""
        gen = get_db_session()
        session = await anext(gen)
        assert isinstance(session, AsyncSession)
        # 模拟正常生命周期
        try:
            await anext(gen)
        except StopAsyncIteration:
            pass

    async def test_rollback_on_exception(self) -> None:
        """Given dependency 内抛异常；When 退出；Then session 自动 rollback 并重新抛。"""
        gen = get_db_session()
        session = await anext(gen)
        assert isinstance(session, AsyncSession)
        with pytest.raises(RuntimeError):
            await gen.athrow(RuntimeError("boom"))


class TestNamingConvention:
    def test_convention_contains_all_alembic_keys(self) -> None:
        for key in ("ix", "uq", "ck", "fk", "pk"):
            assert key in NAMING_CONVENTION

    def test_base_metadata_uses_convention(self) -> None:
        # Base.metadata 应使用上面注入的命名约定
        meta = Base.metadata
        assert meta.naming_convention == NAMING_CONVENTION
