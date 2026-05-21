"""Task 8.3 验收：UploadService 全流程."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.config import get_settings
from app.core.database import Base
from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    TaskClosedForSubmissionError,
    UploadLimitExceededError,
    UploadTooLargeError,
    UploadTooSmallError,
)
from app.services.upload_service import UploadInput, UploadService
from tests.factories.org_factory import (
    ClassFactory,
    CourseFactory,
    MembershipFactory,
)
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import TeacherFactory, UserFactory
from tests.fakes.fake_storage import InMemoryStorage


pytestmark = pytest.mark.unit

# 一个最小 PDF（含 magic）
_PDF_HEAD = b"%PDF-1.4\n"
_VALID_PDF = _PDF_HEAD + b"x" * (2048 - len(_PDF_HEAD))  # 2KB


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
def svc() -> tuple[UploadService, InMemoryStorage]:
    storage = InMemoryStorage()
    return UploadService(storage), storage


@pytest.fixture()
async def setup(session: AsyncSession) -> dict[str, object]:
    teacher = await TeacherFactory.create_async(session, username="t-up")
    course = await CourseFactory.create_async(session)
    cls = await ClassFactory.create_async(session, teacher=teacher, course=course)
    student = await UserFactory.create_async(session, username="s-up")
    await session.commit()
    await MembershipFactory.create_async(session, class_obj=cls, student=student)
    await session.commit()

    # 创建 published 任务
    task = await TrainingTaskFactory.create_async(
        session,
        teacher=teacher,
        course_id=course.id,
        classes=[cls],
        deadline=datetime.now(UTC) + timedelta(days=7),
        with_dimensions=2,
        status="published",
    )
    await session.commit()
    return {
        "teacher": teacher,
        "course": course,
        "class": cls,
        "student": student,
        "task": task,
    }


class TestHappyPath:
    async def test_create_upload_succeeds(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, storage = svc
        upload = await service.create_upload(
            session,
            student=setup["student"],
            task_id=setup["task"].id,
            file=UploadInput(filename="report.pdf", content=_VALID_PDF),
        )
        await session.commit()
        assert upload.id is not None
        assert upload.parse_status == "pending"
        assert upload.file_type == "pdf"
        assert await storage.exists(upload.storage_path)


class TestTaskState:
    async def test_unpublished_task_rejected(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, _ = svc
        # 修改任务为 draft
        setup["task"].status = "draft"
        await session.commit()

        with pytest.raises(TaskClosedForSubmissionError):
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput("x.pdf", _VALID_PDF),
            )

    async def test_past_deadline_rejected(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, _ = svc
        setup["task"].deadline = datetime.now(UTC) - timedelta(hours=1)
        await session.commit()

        with pytest.raises(TaskClosedForSubmissionError):
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput("x.pdf", _VALID_PDF),
            )


class TestSizeValidation:
    async def test_file_too_small(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, _ = svc
        # 0.5KB
        small = _PDF_HEAD + b"x" * 100
        with pytest.raises(UploadTooSmallError):
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput("x.pdf", small),
            )

    async def test_file_too_large(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
        monkeypatch: pytest.MonkeyPatch,
    ) -> None:
        service, _ = svc
        monkeypatch.setenv("TES_MAX_UPLOAD_SIZE_MB", "1")
        get_settings.cache_clear()

        big = _PDF_HEAD + b"x" * (2 * 1024 * 1024)  # 2 MB > 1 MB 限制
        with pytest.raises(UploadTooLargeError):
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput("x.pdf", big),
            )

        get_settings.cache_clear()


class TestPermission:
    async def test_student_not_in_class_rejected(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, _ = svc
        outsider = await UserFactory.create_async(session, username="out")
        await session.commit()

        with pytest.raises(AuthorizationError):
            await service.create_upload(
                session,
                student=outsider,
                task_id=setup["task"].id,
                file=UploadInput("x.pdf", _VALID_PDF),
            )


class TestFileTypeMismatch:
    async def test_exe_renamed_pdf_rejected(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, _ = svc
        evil = b"MZ\x90\x00" + b"x" * 2000  # 改名为 .pdf 的 exe

        with pytest.raises(BusinessRuleError):
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput("malware.pdf", evil),
            )


class TestUploadLimit:
    async def test_21st_upload_rejected(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, _ = svc
        for i in range(20):
            content = _PDF_HEAD + str(i).encode().ljust(1024, b"x")
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput(f"f{i}.pdf", content),
            )
        await session.commit()

        with pytest.raises(UploadLimitExceededError):
            await service.create_upload(
                session,
                student=setup["student"],
                task_id=setup["task"].id,
                file=UploadInput("21.pdf", _VALID_PDF),
            )


class TestSha256Reuse:
    async def test_same_content_reuses_storage_path(
        self,
        session: AsyncSession,
        svc: tuple[UploadService, InMemoryStorage],
        setup: dict[str, object],
    ) -> None:
        service, storage = svc
        u1 = await service.create_upload(
            session,
            student=setup["student"],
            task_id=setup["task"].id,
            file=UploadInput("a.pdf", _VALID_PDF),
        )
        await session.commit()
        u2 = await service.create_upload(
            session,
            student=setup["student"],
            task_id=setup["task"].id,
            file=UploadInput("b.pdf", _VALID_PDF),  # 同内容
        )
        await session.commit()

        assert u1.storage_path == u2.storage_path
        assert u1.sha256 == u2.sha256
        assert u1.id != u2.id
        # storage 中只有一个文件
        assert sum(
            1 for k in storage._store if k == u1.storage_path
        ) == 1
