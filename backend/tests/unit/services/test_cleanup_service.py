"""Task 8.8 验收：文件清理服务."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.services.cleanup_service import (
    cleanup_old_failed_uploads,
    find_orphan_paths,
)
from tests.factories.upload_factory import UploadFactory
from tests.fakes.fake_storage import InMemoryStorage


pytestmark = pytest.mark.unit


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestFindOrphan:
    async def test_identifies_orphans(self, session: AsyncSession) -> None:
        u1 = await UploadFactory.create_async(session)
        u2 = await UploadFactory.create_async(session)
        await session.commit()

        all_paths = {u1.storage_path, u2.storage_path, "orphan/x.pdf"}
        orphans = await find_orphan_paths(session, known_paths=all_paths)
        assert orphans == {"orphan/x.pdf"}

    async def test_no_orphans_when_all_referenced(
        self, session: AsyncSession
    ) -> None:
        u = await UploadFactory.create_async(session)
        await session.commit()
        orphans = await find_orphan_paths(session, known_paths={u.storage_path})
        assert orphans == set()


class TestCleanupOldFailed:
    async def test_deletes_old_failed(self, session: AsyncSession) -> None:
        storage = InMemoryStorage()
        u_old = await UploadFactory.create_async(
            session, parse_status="failed"
        )
        await storage.save(u_old.storage_path, b"data")

        u_recent = await UploadFactory.create_async(
            session, parse_status="failed"
        )
        await storage.save(u_recent.storage_path, b"data")
        await session.commit()

        # 手动设置 u_old 的 updated_at 为很久之前
        from sqlalchemy import update
        from app.models.upload import Upload as U

        far_past = datetime.now(UTC) - timedelta(days=200)
        await session.execute(
            update(U).where(U.id == u_old.id).values(updated_at=far_past)
        )
        await session.commit()

        n = await cleanup_old_failed_uploads(session, storage=storage)
        await session.commit()
        assert n == 1

        await session.refresh(u_old)
        await session.refresh(u_recent)
        assert u_old.is_deleted == 1
        assert u_recent.is_deleted == 0
        assert await storage.exists(u_old.storage_path) is False
        assert await storage.exists(u_recent.storage_path) is True

    async def test_idempotent_on_already_deleted(
        self, session: AsyncSession
    ) -> None:
        storage = InMemoryStorage()
        # 没有 failed 记录
        n = await cleanup_old_failed_uploads(session, storage=storage)
        assert n == 0

    async def test_recent_failed_not_deleted(self, session: AsyncSession) -> None:
        storage = InMemoryStorage()
        recent = await UploadFactory.create_async(session, parse_status="failed")
        await storage.save(recent.storage_path, b"data")
        await session.commit()

        n = await cleanup_old_failed_uploads(session, storage=storage)
        await session.commit()
        assert n == 0
        assert recent.is_deleted == 0
