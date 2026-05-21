"""Task 6.1 验收：EvaluationTemplate / TemplateDimension."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import select, text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.template import EvalTemplate, TemplateDimension
from tests.factories.user_factory import TeacherFactory


pytestmark = pytest.mark.integration


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


class TestEvalTemplate:
    async def test_create_with_dimensions(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t1")
        await session.commit()

        tpl = EvalTemplate(name="代码评价", owner_id=teacher.id)
        session.add(tpl)
        await session.commit()
        for w, name in [(40, "规范"), (40, "功能"), (20, "测试")]:
            session.add(
                TemplateDimension(template_id=tpl.id, name=name, weight=w)
            )
        await session.commit()

        await session.refresh(tpl)
        assert len(tpl.items) == 3

    async def test_visibility_check_constraint(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t2")
        await session.commit()

        tpl = EvalTemplate(name="X", owner_id=teacher.id, visibility="badvalue")
        session.add(tpl)
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()

    async def test_duplicate_name_allowed_per_owner(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="t3")
        await session.commit()

        for _ in range(2):
            session.add(EvalTemplate(name="同名", owner_id=teacher.id))
        await session.commit()

        rows = (
            await session.execute(select(EvalTemplate).where(EvalTemplate.name == "同名"))
        ).scalars().all()
        assert len(rows) == 2


class TestCascadeDelete:
    async def test_delete_template_removes_dimensions(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="t4")
        await session.commit()

        tpl = EvalTemplate(name="DELME", owner_id=teacher.id)
        session.add(tpl)
        await session.commit()
        for i in range(3):
            session.add(TemplateDimension(template_id=tpl.id, name=f"d{i}", weight=10))
        await session.commit()
        tpl_id = tpl.id

        await session.delete(tpl)
        await session.commit()

        remaining = (
            await session.execute(
                select(TemplateDimension).where(TemplateDimension.template_id == tpl_id)
            )
        ).scalars().all()
        assert len(remaining) == 0


class TestSystemTemplate:
    async def test_system_template_owner_can_be_null(
        self, session: AsyncSession
    ) -> None:
        tpl = EvalTemplate(
            name="系统模板", owner_id=None, visibility="system"
        )
        session.add(tpl)
        await session.commit()
        assert tpl.id is not None
        assert tpl.owner_id is None
