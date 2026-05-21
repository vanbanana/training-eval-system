"""Task 2.3 验收：通用 Repository 基类."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import Column, Integer, String
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.repositories.base import BaseRepository


pytestmark = pytest.mark.integration


# 测试模型 - 仅用于本测试文件
class _DummyItem(Base):
    __tablename__ = "_dummy_items"
    id = Column(Integer, primary_key=True)
    name = Column(String(50), nullable=False)


class _DummyRepo(BaseRepository[_DummyItem]):
    model = _DummyItem


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    """每个测试一个独立 in-memory SQLite。"""
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(_DummyItem.__table__.create)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestCRUD:
    async def test_create_then_get(self, session: AsyncSession) -> None:
        """Given 空表；When create+get；Then 返回完整对象。"""
        repo = _DummyRepo()
        item = await repo.create(session, name="alpha")
        assert item.id is not None

        loaded = await repo.get(session, item.id)
        assert loaded is not None
        assert loaded.name == "alpha"

    async def test_get_nonexistent_returns_none(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        assert await repo.get(session, 9999) is None

    async def test_list_with_pagination(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        for i in range(5):
            await repo.create(session, name=f"n{i}")

        # 默认前 50
        all_items = await repo.list(session)
        assert len(all_items) == 5

        # 分页
        page = await repo.list(session, offset=2, limit=2)
        assert len(page) == 2

    async def test_count(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        assert await repo.count(session) == 0
        await repo.create(session, name="a")
        await repo.create(session, name="b")
        assert await repo.count(session) == 2

    async def test_update_returns_affected_rows(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        item = await repo.create(session, name="old")
        n = await repo.update(session, item.id, name="new")
        assert n == 1

        loaded = await repo.get(session, item.id)
        assert loaded is not None
        await session.refresh(loaded)
        assert loaded.name == "new"

    async def test_update_nonexistent_returns_zero(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        n = await repo.update(session, 9999, name="x")
        assert n == 0

    async def test_update_with_no_fields_returns_zero(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        item = await repo.create(session, name="x")
        n = await repo.update(session, item.id)
        assert n == 0

    async def test_delete_returns_affected_rows(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        item = await repo.create(session, name="bye")
        n = await repo.delete(session, item.id)
        assert n == 1

        assert await repo.get(session, item.id) is None

    async def test_delete_nonexistent_returns_zero(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        n = await repo.delete(session, 99999)
        assert n == 0

    async def test_exists(self, session: AsyncSession) -> None:
        repo = _DummyRepo()
        item = await repo.create(session, name="check")
        assert await repo.exists(session, item.id) is True
        assert await repo.exists(session, 9999) is False
