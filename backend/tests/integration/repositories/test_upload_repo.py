"""Task 8.2 验收：UploadRepository."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import ConflictError
from app.models.upload import Upload
from app.repositories.upload_repo import UploadRepository
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory


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


class TestSha256Lookup:
    async def test_find_in_same_student_only(
        self, session: AsyncSession
    ) -> None:
        repo = UploadRepository()
        s1 = await UserFactory.create_async(session, username="s1")
        s2 = await UserFactory.create_async(session, username="s2")
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        sha = "a" * 64
        for student in (s1, s2):
            session.add(
                Upload(
                    task_id=task.id,
                    student_id=student.id,
                    filename="x.pdf",
                    file_type="pdf",
                    file_size=10,
                    storage_path=f"p/{student.id}",
                    sha256=sha,
                )
            )
        await session.commit()

        # s1 范围内能找到自己
        found_s1 = await repo.find_by_sha256(session, student_id=s1.id, sha256=sha)
        assert found_s1 is not None
        assert found_s1.student_id == s1.id

        # 跨学生不冲突：仅返回 s1 自己的
        assert found_s1.student_id != s2.id


class TestStatusTransition:
    async def test_valid_transition_pending_to_parsing(
        self, session: AsyncSession
    ) -> None:
        repo = UploadRepository()
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        u = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="p",
            parse_status="pending",
        )
        session.add(u)
        await session.commit()

        updated = await repo.update_status(session, u.id, "parsing")
        await session.commit()
        assert updated.parse_status == "parsing"

    async def test_invalid_transition_rejected(
        self, session: AsyncSession
    ) -> None:
        """parsed → pending 是非法的。"""
        repo = UploadRepository()
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        u = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="p",
            parse_status="parsed",
        )
        session.add(u)
        await session.commit()

        with pytest.raises(ConflictError):
            await repo.update_status(session, u.id, "pending")


class TestListAndCount:
    async def test_count_by_task_student(self, session: AsyncSession) -> None:
        repo = UploadRepository()
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        for i in range(3):
            session.add(
                Upload(
                    task_id=task.id,
                    student_id=student.id,
                    filename=f"f{i}.pdf",
                    file_type="pdf",
                    file_size=10,
                    storage_path=f"p{i}",
                    version=i + 1,
                )
            )
        await session.commit()

        n = await repo.count_by_task_student(
            session, task_id=task.id, student_id=student.id
        )
        assert n == 3

    async def test_soft_delete_excludes_from_list(
        self, session: AsyncSession
    ) -> None:
        repo = UploadRepository()
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        u = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="p",
        )
        session.add(u)
        await session.commit()

        await repo.soft_delete(session, u.id)
        await session.commit()

        items = await repo.list_by_task_student(
            session, task_id=task.id, student_id=student.id
        )
        assert len(items) == 0
