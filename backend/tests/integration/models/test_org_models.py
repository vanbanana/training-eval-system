"""Task 4.1 验收：Course / Class / ClassMembership 模型."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.course import Class, ClassMembership, Course
from tests.factories.user_factory import TeacherFactory, UserFactory


pytestmark = pytest.mark.integration


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    # SQLite 默认不强制 FK；启用以测试 cascade
    async with engine.begin() as conn:
        await conn.execute(__import__("sqlalchemy").text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestCourseModel:
    async def test_create_course(self, session: AsyncSession) -> None:
        c = Course(name="软件工程", code="SE2024")
        session.add(c)
        await session.commit()
        assert c.id is not None
        assert c.is_archived is False

    async def test_code_unique(self, session: AsyncSession) -> None:
        session.add(Course(name="A", code="DUP"))
        await session.commit()
        session.add(Course(name="B", code="DUP"))
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestClassModel:
    async def test_create_class_with_teacher_and_course(
        self, session: AsyncSession
    ) -> None:
        course = Course(name="DS", code="DS01")
        teacher = await TeacherFactory.create_async(session, username="t")
        session.add(course)
        await session.commit()

        cls = Class(name="DS-21-1", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()
        assert cls.id is not None
        assert cls.student_count == 0


class TestClassMembership:
    async def test_join_class(self, session: AsyncSession) -> None:
        course = Course(name="C", code="C001")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t2")
        student = await UserFactory.create_async(session, username="s1")
        await session.commit()

        cls = Class(name="C-1", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()

        m = ClassMembership(class_id=cls.id, student_id=student.id)
        session.add(m)
        await session.commit()
        assert m.id is not None
        assert m.joined_at is not None

    async def test_unique_membership(self, session: AsyncSession) -> None:
        course = Course(name="C", code="C002")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t3")
        student = await UserFactory.create_async(session, username="s2")
        await session.commit()
        cls = Class(name="X", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()

        session.add(ClassMembership(class_id=cls.id, student_id=student.id))
        await session.commit()
        # 重复加入
        session.add(ClassMembership(class_id=cls.id, student_id=student.id))
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()

    async def test_cascade_delete_class_removes_memberships(
        self, session: AsyncSession
    ) -> None:
        course = Course(name="C", code="C003")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t4")
        students = [
            await UserFactory.create_async(session, username=f"sd{i}") for i in range(3)
        ]
        await session.commit()
        cls = Class(name="X3", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()

        for s in students:
            session.add(ClassMembership(class_id=cls.id, student_id=s.id))
        await session.commit()

        # 删 class → memberships 级联删除
        await session.delete(cls)
        await session.commit()

        rows = (
            await session.execute(
                select(ClassMembership).where(ClassMembership.class_id == cls.id)
            )
        ).scalars().all()
        assert len(rows) == 0
