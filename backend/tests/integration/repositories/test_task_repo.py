"""Task 5.2 验收：TaskRepository / DimensionRepository."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.task import Dimension, TrainingTask
from app.repositories.task_repo import DimensionRepository, TaskRepository
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


class TestTaskRepository:
    async def test_list_by_teacher_filters_correctly(
        self, session: AsyncSession
    ) -> None:
        ta = await TeacherFactory.create_async(session, username="ta")
        tb = await TeacherFactory.create_async(session, username="tb")
        course = await CourseFactory.create_async(session)
        await session.commit()

        for _ in range(3):
            session.add(
                TrainingTask(name="A", teacher_id=ta.id, course_id=course.id)
            )
        session.add(TrainingTask(name="B", teacher_id=tb.id, course_id=course.id))
        await session.commit()

        repo = TaskRepository()
        a_tasks = await repo.list_by_teacher(session, ta.id)
        assert len(a_tasks) == 3
        b_tasks = await repo.list_by_teacher(session, tb.id)
        assert len(b_tasks) == 1

    async def test_list_by_teacher_with_status_filter(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="ts")
        course = await CourseFactory.create_async(session)
        await session.commit()

        session.add(
            TrainingTask(
                name="d", teacher_id=teacher.id, course_id=course.id, status="draft"
            )
        )
        session.add(
            TrainingTask(
                name="p", teacher_id=teacher.id, course_id=course.id, status="published"
            )
        )
        session.add(
            TrainingTask(
                name="c", teacher_id=teacher.id, course_id=course.id, status="closed"
            )
        )
        await session.commit()

        repo = TaskRepository()
        published = await repo.list_by_teacher(session, teacher.id, status="published")
        assert len(published) == 1

    async def test_list_by_class(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="tc")
        course = await CourseFactory.create_async(session)
        cls1 = await ClassFactory.create_async(session, teacher=teacher, course=course)
        cls2 = await ClassFactory.create_async(session, teacher=teacher, course=course)
        await session.commit()

        t1 = TrainingTask(
            name="t1", teacher_id=teacher.id, course_id=course.id, status="published"
        )
        t1.classes = [cls1]
        session.add(t1)

        t2 = TrainingTask(
            name="t2", teacher_id=teacher.id, course_id=course.id, status="published"
        )
        t2.classes = [cls1, cls2]
        session.add(t2)
        await session.commit()

        repo = TaskRepository()
        cls1_tasks = await repo.list_by_class(session, cls1.id)
        assert len(cls1_tasks) == 2
        cls2_tasks = await repo.list_by_class(session, cls2.id)
        assert len(cls2_tasks) == 1


class TestDimensionRepository:
    async def test_list_by_task_ordered(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="td")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(name="X", teacher_id=teacher.id, course_id=course.id)
        session.add(task)
        await session.commit()

        for w, idx, name in [(20, 2, "C"), (30, 0, "A"), (50, 1, "B")]:
            session.add(
                Dimension(task_id=task.id, name=name, weight=w, order_index=idx)
            )
        await session.commit()

        repo = DimensionRepository()
        dims = await repo.list_by_task(session, task.id)
        names = [d.name for d in dims]
        assert names == ["A", "B", "C"]

    async def test_sum_weights(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="tw")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(name="X", teacher_id=teacher.id, course_id=course.id)
        session.add(task)
        await session.commit()

        repo = DimensionRepository()
        # 空时返回 0
        assert await repo.sum_weights(session, task.id) == 0

        for w in [30, 30, 40]:
            session.add(Dimension(task_id=task.id, name="d", weight=w))
        await session.commit()

        assert await repo.sum_weights(session, task.id) == 100

    async def test_replace_all_for_task_atomic(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="tr")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(name="X", teacher_id=teacher.id, course_id=course.id)
        session.add(task)
        await session.commit()

        repo = DimensionRepository()

        # 第一次添加 3 个
        await repo.replace_all_for_task(
            session,
            task.id,
            [
                {"name": "A", "weight": 30},
                {"name": "B", "weight": 30},
                {"name": "C", "weight": 40},
            ],
        )
        await session.commit()
        assert await repo.sum_weights(session, task.id) == 100
        dims = await repo.list_by_task(session, task.id)
        assert len(dims) == 3

        # 替换为 2 个
        await repo.replace_all_for_task(
            session,
            task.id,
            [
                {"name": "X", "weight": 60},
                {"name": "Y", "weight": 40},
            ],
        )
        await session.commit()
        dims2 = await repo.list_by_task(session, task.id)
        assert len(dims2) == 2
        assert {d.name for d in dims2} == {"X", "Y"}

    async def test_replace_all_rolls_back_on_failure(
        self, session: AsyncSession
    ) -> None:
        """Given replace_all 中途异常；When rollback；Then DB 状态为 replace 之前."""
        teacher = await TeacherFactory.create_async(session, username="trf")
        course = await CourseFactory.create_async(session)
        await session.commit()
        task = TrainingTask(name="X", teacher_id=teacher.id, course_id=course.id)
        session.add(task)
        await session.commit()

        repo = DimensionRepository()
        # 先建 1 个维度
        await repo.replace_all_for_task(
            session, task.id, [{"name": "Old", "weight": 100}]
        )
        await session.commit()

        # 用 SAVEPOINT 隔离失败：可释放后继续使用 session
        from sqlalchemy.exc import IntegrityError

        savepoint = await session.begin_nested()
        try:
            await repo.replace_all_for_task(
                session,
                task.id,
                [{"name": "Bad", "weight": 0}],  # CHECK 失败
            )
            await savepoint.commit()
            pytest.fail("应抛出 IntegrityError")
        except IntegrityError:
            await savepoint.rollback()

        # 原状态保留
        dims = await repo.list_by_task(session, task.id)
        assert len(dims) == 1
        assert dims[0].name == "Old"
