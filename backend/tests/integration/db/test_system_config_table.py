"""Task 2.4 验收：system_config 表迁移与模型."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.system_config import SystemConfig


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


class TestSystemConfigTable:
    async def test_can_insert_and_query(self, session: AsyncSession) -> None:
        cfg = SystemConfig(
            key="evaluation.objective_ratio",
            value=0.7,
            category="evaluation",
            description="客观分占比",
        )
        session.add(cfg)
        await session.commit()

        from sqlalchemy import select
        loaded = (
            await session.execute(
                select(SystemConfig).where(SystemConfig.key == "evaluation.objective_ratio")
            )
        ).scalar_one()
        assert loaded.value == 0.7
        assert loaded.category == "evaluation"

    async def test_jsonb_supports_complex_types(self, session: AsyncSession) -> None:
        """Given JSON 字段；When 写数组/对象；Then 可还原。"""
        cfg = SystemConfig(
            key="grading.dimensions",
            value=[{"name": "代码", "weight": 30}, {"name": "报告", "weight": 70}],
            category="grading",
        )
        session.add(cfg)
        await session.commit()

        await session.refresh(cfg)
        assert cfg.value == [{"name": "代码", "weight": 30}, {"name": "报告", "weight": 70}]

    async def test_key_uniqueness_constraint(self, session: AsyncSession) -> None:
        session.add(SystemConfig(key="dup.key", value=1, category="x"))
        await session.commit()

        session.add(SystemConfig(key="dup.key", value=2, category="x"))
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()

    async def test_default_category_is_general(self, session: AsyncSession) -> None:
        cfg = SystemConfig(key="some.key", value="hello")
        session.add(cfg)
        await session.commit()
        await session.refresh(cfg)
        assert cfg.category == "general"

    async def test_updated_at_auto_set(self, session: AsyncSession) -> None:
        cfg = SystemConfig(key="ts.key", value=1)
        session.add(cfg)
        await session.commit()
        await session.refresh(cfg)
        assert cfg.updated_at is not None
