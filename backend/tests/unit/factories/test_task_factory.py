"""Task 5.6 验收：TrainingTaskFactory / DimensionFactory."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.repositories.task_repo import DimensionRepository
from tests.factories.task_factory import DimensionFactory, TrainingTaskFactory


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


class TestTrainingTaskFactory:
    async def test_default_task_published_with_future_deadline(
        self, session: AsyncSession
    ) -> None:
        task = await TrainingTaskFactory.create_async(session)
        assert task.id is not None
        assert task.status == "published"
        assert task.deadline is not None
        deadline = task.deadline
        if deadline.tzinfo is None:
            deadline = deadline.replace(tzinfo=UTC)
        assert deadline > datetime.now(UTC)

    async def test_with_dimensions_4_sums_to_100(self, session: AsyncSession) -> None:
        task = await TrainingTaskFactory.create_async(session, with_dimensions=4)
        await session.commit()

        repo = DimensionRepository()
        total = await repo.sum_weights(session, task.id)
        assert total == 100
        dims = await repo.list_by_task(session, task.id)
        assert len(dims) == 4

    async def test_with_dimensions_3_sums_to_100(self, session: AsyncSession) -> None:
        task = await TrainingTaskFactory.create_async(session, with_dimensions=3)
        await session.commit()
        repo = DimensionRepository()
        assert await repo.sum_weights(session, task.id) == 100

    async def test_default_links_one_class(self, session: AsyncSession) -> None:
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        await session.refresh(task)
        assert len(task.classes) == 1


class TestDimensionFactory:
    async def test_creates_unique_dimensions(self, session: AsyncSession) -> None:
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        d1 = await DimensionFactory.create_async(
            session, task=task, weight=30
        )
        d2 = await DimensionFactory.create_async(
            session, task=task, weight=70
        )
        await session.commit()

        assert d1.order_index != d2.order_index
        assert d1.name != d2.name
