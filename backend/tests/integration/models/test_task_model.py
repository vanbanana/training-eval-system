"""Task 5.1 验收：TrainingTask 与 Dimension 模型."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.task import Dimension, TrainingTask, task_classes
from tests.factories.org_factory import ClassFactory, CourseFactory
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


class TestTrainingTaskSchema:
    async def test_create_default_draft(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t")
        course = await CourseFactory.create_async(session)
        await session.commit()

        task = TrainingTask(
            name="实训 1", teacher_id=teacher.id, course_id=course.id
        )
        session.add(task)
        await session.commit()
        assert task.id is not None
        assert task.status == "draft"

    async def test_status_check_constraint(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t2")
        course = await CourseFactory.create_async(session)
        await session.commit()

        task = TrainingTask(
            name="X",
            teacher_id=teacher.id,
            course_id=course.id,
            status="invalid",
        )
        session.add(task)
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestDimensionConstraints:
    async def test_weight_in_range_passes(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t3")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(
            name="X", teacher_id=teacher.id, course_id=course.id
        )
        session.add(task)
        await session.commit()

        for w in (1, 50, 100):
            session.add(Dimension(task_id=task.id, name=f"d{w}", weight=w))
        await session.commit()

    async def test_weight_zero_rejected(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t4")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(
            name="X", teacher_id=teacher.id, course_id=course.id
        )
        session.add(task)
        await session.commit()

        session.add(Dimension(task_id=task.id, name="d", weight=0))
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()

    async def test_weight_above_100_rejected(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t5")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(
            name="X", teacher_id=teacher.id, course_id=course.id
        )
        session.add(task)
        await session.commit()

        session.add(Dimension(task_id=task.id, name="d", weight=101))
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestTaskClassesAssociation:
    async def test_link_task_to_classes(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="t6")
        course = await CourseFactory.create_async(session)
        cls1 = await ClassFactory.create_async(session, teacher=teacher, course=course)
        cls2 = await ClassFactory.create_async(session, teacher=teacher, course=course)
        await session.commit()

        task = TrainingTask(
            name="X",
            teacher_id=teacher.id,
            course_id=course.id,
            deadline=datetime.now(UTC) + timedelta(days=7),
        )
        task.classes = [cls1, cls2]
        session.add(task)
        await session.commit()
        await session.refresh(task)

        assert len(task.classes) == 2

    async def test_cascade_delete_task_removes_associations(
        self, session: AsyncSession
    ) -> None:
        from sqlalchemy import select

        teacher = await TeacherFactory.create_async(session, username="t7")
        course = await CourseFactory.create_async(session)
        cls = await ClassFactory.create_async(session, teacher=teacher, course=course)
        await session.commit()
        task = TrainingTask(name="X", teacher_id=teacher.id, course_id=course.id)
        task.classes = [cls]
        session.add(task)
        await session.commit()
        task_id = task.id

        await session.delete(task)
        await session.commit()

        rows = await session.execute(
            select(task_classes).where(task_classes.c.task_id == task_id)
        )
        assert rows.first() is None


class TestCascadeDeleteDimensions:
    async def test_delete_task_removes_dimensions(
        self, session: AsyncSession
    ) -> None:
        from sqlalchemy import select

        teacher = await TeacherFactory.create_async(session, username="t8")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(name="X", teacher_id=teacher.id, course_id=course.id)
        session.add(task)
        await session.commit()
        for i in range(3):
            session.add(Dimension(task_id=task.id, name=f"d{i}", weight=10))
        await session.commit()
        task_id = task.id

        await session.delete(task)
        await session.commit()

        remaining = (
            await session.execute(
                select(Dimension).where(Dimension.task_id == task_id)
            )
        ).scalars().all()
        assert len(remaining) == 0
