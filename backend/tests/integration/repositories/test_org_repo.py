"""Task 4.2 验收：OrgRepository（课程/班级/成员）."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.course import Class, ClassMembership, Course
from app.repositories.org_repo import (
    ClassRepository,
    CourseRepository,
    MembershipRepository,
)
from tests.factories.user_factory import TeacherFactory, UserFactory


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


class TestCourseRepository:
    async def test_get_by_code_finds_existing(self, session: AsyncSession) -> None:
        repo = CourseRepository()
        c = await repo.create(session, name="X", code="XYZ")
        await session.commit()

        found = await repo.get_by_code(session, "XYZ")
        assert found is not None
        assert found.id == c.id

    async def test_get_by_code_returns_none(self, session: AsyncSession) -> None:
        repo = CourseRepository()
        assert await repo.get_by_code(session, "NOPE") is None

    async def test_list_active_excludes_archived(self, session: AsyncSession) -> None:
        repo = CourseRepository()
        await repo.create(session, name="A", code="A1")
        await repo.create(session, name="B", code="B1", is_archived=True)
        await session.commit()

        active = await repo.list_active(session)
        codes = {c.code for c in active}
        assert "A1" in codes
        assert "B1" not in codes


class TestMembershipRepository:
    async def test_is_student_in_class_true(self, session: AsyncSession) -> None:
        course = Course(name="C", code="C-A")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t")
        student = await UserFactory.create_async(session, username="s")
        await session.commit()
        cls = Class(name="X", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()
        session.add(ClassMembership(class_id=cls.id, student_id=student.id))
        await session.commit()

        m_repo = MembershipRepository()
        assert (
            await m_repo.is_student_in_class(
                session, student_id=student.id, class_id=cls.id
            )
            is True
        )

    async def test_is_student_in_class_false(self, session: AsyncSession) -> None:
        m_repo = MembershipRepository()
        assert (
            await m_repo.is_student_in_class(session, student_id=999, class_id=999)
            is False
        )

    async def test_archived_class_still_returns_true(
        self, session: AsyncSession
    ) -> None:
        course = Course(name="C2", code="C-B")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t2")
        student = await UserFactory.create_async(session, username="s2")
        await session.commit()
        cls = Class(
            name="Y",
            course_id=course.id,
            teacher_id=teacher.id,
            is_archived=True,
        )
        session.add(cls)
        await session.commit()
        session.add(ClassMembership(class_id=cls.id, student_id=student.id))
        await session.commit()

        m_repo = MembershipRepository()
        assert (
            await m_repo.is_student_in_class(
                session, student_id=student.id, class_id=cls.id
            )
            is True
        )

    async def test_list_students_of_class(self, session: AsyncSession) -> None:
        course = Course(name="C3", code="C-C")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t3")
        students = [
            await UserFactory.create_async(session, username=f"st{i}")
            for i in range(3)
        ]
        await session.commit()
        cls = Class(name="L", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()
        for s in students:
            session.add(ClassMembership(class_id=cls.id, student_id=s.id))
        await session.commit()

        m_repo = MembershipRepository()
        members = await m_repo.list_students_of_class(session, cls.id)
        assert len(members) == 3

    async def test_remove_membership(self, session: AsyncSession) -> None:
        course = Course(name="C4", code="C-D")
        session.add(course)
        teacher = await TeacherFactory.create_async(session, username="t4")
        student = await UserFactory.create_async(session, username="s4")
        await session.commit()
        cls = Class(name="Z", course_id=course.id, teacher_id=teacher.id)
        session.add(cls)
        await session.commit()
        session.add(ClassMembership(class_id=cls.id, student_id=student.id))
        await session.commit()

        m_repo = MembershipRepository()
        n = await m_repo.remove_membership(
            session, student_id=student.id, class_id=cls.id
        )
        await session.commit()
        assert n == 1
        assert (
            await m_repo.is_student_in_class(
                session, student_id=student.id, class_id=cls.id
            )
            is False
        )


class TestClassRepository:
    async def test_list_by_teacher(self, session: AsyncSession) -> None:
        c_repo = ClassRepository()
        course = Course(name="X", code="XX")
        session.add(course)
        t1 = await TeacherFactory.create_async(session, username="t1")
        t2 = await TeacherFactory.create_async(session, username="t2-other")
        await session.commit()

        for _ in range(2):
            session.add(Class(name="A", course_id=course.id, teacher_id=t1.id))
        session.add(Class(name="B", course_id=course.id, teacher_id=t2.id))
        await session.commit()

        cs = await c_repo.list_by_teacher(session, t1.id)
        assert len(cs) == 2
