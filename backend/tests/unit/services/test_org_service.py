"""Task 4.3 验收：OrgService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import AuthorizationError, ConflictError, ResourceNotFoundError
from app.services.org_service import OrgService
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


@pytest.fixture()
def svc() -> OrgService:
    return OrgService()


class TestCreateCourse:
    async def test_admin_can_create(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="ad")
        await session.commit()
        c = await svc.create_course(session, actor=admin, name="DS", code="DS01")
        await session.commit()
        assert c.id is not None

    async def test_teacher_rejected(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="t")
        await session.commit()
        with pytest.raises(AuthorizationError):
            await svc.create_course(session, actor=teacher, name="X", code="X01")

    async def test_duplicate_code_raises_conflict(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="ad2")
        await session.commit()
        await svc.create_course(session, actor=admin, name="A", code="DUP")
        await session.commit()
        with pytest.raises(ConflictError) as exc:
            await svc.create_course(session, actor=admin, name="B", code="DUP")
        assert exc.value.field == "code"


class TestCreateClass:
    async def test_teacher_creates_class_for_self(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="adc")
        teacher = await TeacherFactory.create_async(session, username="tc")
        await session.commit()
        course = await svc.create_course(session, actor=admin, name="C", code="C-T")
        await session.commit()

        cls = await svc.create_class(
            session, actor=teacher, name="X-1", course_id=course.id
        )
        await session.commit()
        assert cls.teacher_id == teacher.id

    async def test_teacher_cannot_assign_other_teacher(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        """Given teacher A 创建班级；When 指定 teacher_id=B；Then 仍归 A 所有。"""
        admin = await AdminFactory.create_async(session, username="aa")
        ta = await TeacherFactory.create_async(session, username="ta")
        tb = await TeacherFactory.create_async(session, username="tb")
        await session.commit()
        course = await svc.create_course(session, actor=admin, name="C", code="C-X")
        await session.commit()

        cls = await svc.create_class(
            session, actor=ta, name="X-2", course_id=course.id, teacher_id=tb.id
        )
        await session.commit()
        # 实际归 ta（自己）
        assert cls.teacher_id == ta.id

    async def test_admin_can_assign_teacher(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="adm")
        teacher = await TeacherFactory.create_async(session, username="tar")
        await session.commit()
        course = await svc.create_course(session, actor=admin, name="C", code="C-Y")
        await session.commit()

        cls = await svc.create_class(
            session, actor=admin, name="X-3", course_id=course.id, teacher_id=teacher.id
        )
        await session.commit()
        assert cls.teacher_id == teacher.id


class TestBulkAddStudents:
    async def test_add_with_mixed_valid_invalid(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        """Given 4 个学生 ID（3 个学生 + 1 个不存在 + 1 个教师）；
        When bulk_add；Then 仅有效项被加入，无效项作为 result.failed 返回（不抛异常）。"""
        admin = await AdminFactory.create_async(session, username="adb")
        teacher = await TeacherFactory.create_async(session, username="tb")
        students = [
            await UserFactory.create_async(session, username=f"sb{i}")
            for i in range(3)
        ]
        # 一个老师（非学生角色）
        another_teacher = await TeacherFactory.create_async(
            session, username="tc-other"
        )
        await session.commit()
        course = await svc.create_course(session, actor=admin, name="C", code="C-B")
        await session.commit()
        cls = await svc.create_class(
            session, actor=admin, name="L", course_id=course.id, teacher_id=teacher.id
        )
        await session.commit()

        result = await svc.bulk_add_students(
            session,
            actor=teacher,
            class_id=cls.id,
            student_ids=[
                students[0].id,
                students[1].id,
                students[2].id,
                99999,  # 不存在
                another_teacher.id,  # 非学生
            ],
        )
        await session.commit()

        assert len(result.added) == 3
        assert len(result.failed) == 2
        # 失败原因明确
        reasons = [r[1] for r in result.failed]
        assert any("不存在" in r for r in reasons)
        assert any("非学生" in r for r in reasons)

    async def test_duplicate_student_ignored(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="adup")
        teacher = await TeacherFactory.create_async(session, username="tup")
        student = await UserFactory.create_async(session, username="sup")
        await session.commit()
        course = await svc.create_course(session, actor=admin, name="C", code="C-Du")
        await session.commit()
        cls = await svc.create_class(
            session, actor=admin, name="L", course_id=course.id, teacher_id=teacher.id
        )
        await session.commit()

        # 第一次添加
        r1 = await svc.bulk_add_students(
            session, actor=teacher, class_id=cls.id, student_ids=[student.id]
        )
        await session.commit()
        assert len(r1.added) == 1

        # 重复添加
        r2 = await svc.bulk_add_students(
            session, actor=teacher, class_id=cls.id, student_ids=[student.id]
        )
        await session.commit()
        assert len(r2.added) == 0
        assert len(r2.failed) == 1
        assert "已在班级中" in r2.failed[0][1]


class TestArchiveCourse:
    async def test_archive_marks_archived(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="ada")
        await session.commit()
        c = await svc.create_course(session, actor=admin, name="A", code="ARCH")
        await session.commit()
        result = await svc.archive_course(session, actor=admin, course_id=c.id)
        await session.commit()
        assert result.is_archived is True

    async def test_archive_nonexistent_raises(
        self, session: AsyncSession, svc: OrgService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="adx")
        await session.commit()
        with pytest.raises(ResourceNotFoundError):
            await svc.archive_course(session, actor=admin, course_id=99999)
