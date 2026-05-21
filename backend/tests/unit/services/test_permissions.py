"""Task 4.6 验收：班级归属校验工具."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import AuthorizationError, ResourceNotFoundError
from app.services.permissions import (
    assert_student_in_class,
    assert_teacher_owns_class,
    is_student_in_class,
)
from tests.factories.org_factory import ClassFactory, MembershipFactory
from tests.factories.user_factory import (
    AdminFactory,
    TeacherFactory,
    UserFactory,
)


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


class TestAssertStudentInClass:
    async def test_member_passes(self, session: AsyncSession) -> None:
        cls = await ClassFactory.create_async(session)
        student = await UserFactory.create_async(session)
        await session.commit()
        await MembershipFactory.create_async(
            session, class_obj=cls, student=student
        )
        await session.commit()

        # 不抛异常
        await assert_student_in_class(
            session, student_id=student.id, class_id=cls.id
        )

    async def test_non_member_raises(self, session: AsyncSession) -> None:
        cls = await ClassFactory.create_async(session)
        student = await UserFactory.create_async(session)
        await session.commit()

        with pytest.raises(AuthorizationError) as exc:
            await assert_student_in_class(
                session, student_id=student.id, class_id=cls.id
            )
        assert exc.value.field == "class_id"


class TestAssertTeacherOwnsClass:
    async def test_owner_teacher_passes(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="own")
        cls = await ClassFactory.create_async(session, teacher=teacher)
        await session.commit()

        result = await assert_teacher_owns_class(
            session, teacher=teacher, class_id=cls.id
        )
        assert result.id == cls.id

    async def test_other_teacher_rejected(self, session: AsyncSession) -> None:
        owner = await TeacherFactory.create_async(session, username="o2")
        other = await TeacherFactory.create_async(session, username="other")
        cls = await ClassFactory.create_async(session, teacher=owner)
        await session.commit()

        with pytest.raises(AuthorizationError):
            await assert_teacher_owns_class(
                session, teacher=other, class_id=cls.id
            )

    async def test_admin_always_passes(self, session: AsyncSession) -> None:
        owner = await TeacherFactory.create_async(session, username="o3")
        admin = await AdminFactory.create_async(session, username="a3")
        cls = await ClassFactory.create_async(session, teacher=owner)
        await session.commit()

        result = await assert_teacher_owns_class(
            session, teacher=admin, class_id=cls.id
        )
        assert result.id == cls.id

    async def test_class_not_found_raises(self, session: AsyncSession) -> None:
        teacher = await TeacherFactory.create_async(session, username="o4")
        await session.commit()
        with pytest.raises(ResourceNotFoundError):
            await assert_teacher_owns_class(
                session, teacher=teacher, class_id=99999
            )


class TestIsStudentInClass:
    async def test_returns_bool(self, session: AsyncSession) -> None:
        cls = await ClassFactory.create_async(session)
        student = await UserFactory.create_async(session)
        await session.commit()

        assert (
            await is_student_in_class(
                session, student_id=student.id, class_id=cls.id
            )
            is False
        )
        await MembershipFactory.create_async(
            session, class_obj=cls, student=student
        )
        await session.commit()
        assert (
            await is_student_in_class(
                session, student_id=student.id, class_id=cls.id
            )
            is True
        )
