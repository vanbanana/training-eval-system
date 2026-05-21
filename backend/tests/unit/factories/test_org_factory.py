"""Task 4.5 验收：Course / Class / Membership factories."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from tests.factories.org_factory import (
    ClassFactory,
    CourseFactory,
    MembershipFactory,
)
from tests.factories.user_factory import UserFactory


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


class TestCourseFactory:
    async def test_default(self, session: AsyncSession) -> None:
        c = await CourseFactory.create_async(session)
        assert c.id is not None
        assert c.is_archived is False
        assert c.code.startswith("C")

    async def test_unique_codes(self, session: AsyncSession) -> None:
        codes = set()
        for _ in range(5):
            c = await CourseFactory.create_async(session)
            codes.add(c.code)
        assert len(codes) == 5


class TestClassFactory:
    async def test_default_creates_dependencies(self, session: AsyncSession) -> None:
        cls = await ClassFactory.create_async(session)
        assert cls.id is not None
        assert cls.course_id is not None
        assert cls.teacher_id is not None

    async def test_with_explicit_course_and_teacher(
        self, session: AsyncSession
    ) -> None:
        course = await CourseFactory.create_async(session, name="特定课程")
        from tests.factories.user_factory import TeacherFactory

        teacher = await TeacherFactory.create_async(session, username="explicit-t")

        cls = await ClassFactory.create_async(session, course=course, teacher=teacher)
        assert cls.course_id == course.id
        assert cls.teacher_id == teacher.id


class TestMembershipFactory:
    async def test_creates_relation(self, session: AsyncSession) -> None:
        cls = await ClassFactory.create_async(session)
        student = await UserFactory.create_async(session)
        await session.commit()

        m = await MembershipFactory.create_async(
            session, class_obj=cls, student=student
        )
        assert m.id is not None
        assert m.class_id == cls.id
        assert m.student_id == student.id
