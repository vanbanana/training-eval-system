"""Task 8.1 验收：Upload / ParseResult / VerifyResult."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.upload import ParseResult, Upload, VerifyResult
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


class TestUploadSchema:
    async def test_default_status_pending(self, session: AsyncSession) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="report.pdf",
            file_type="pdf",
            file_size=1024,
            storage_path="task_1/student_1/abc.pdf",
            sha256="x" * 64,
        )
        session.add(upload)
        await session.commit()
        assert upload.parse_status == "pending"

    async def test_invalid_status_rejected(self, session: AsyncSession) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        u = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="x",
            parse_status="bogus",
        )
        session.add(u)
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestParseResultUniqueness:
    async def test_one_parse_result_per_upload(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="x",
        )
        session.add(upload)
        await session.commit()

        session.add(ParseResult(upload_id=upload.id, raw_text="A"))
        await session.commit()

        # 第二个 ParseResult 与同一 upload 关联：违反 UK
        session.add(ParseResult(upload_id=upload.id, raw_text="B"))
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestEmbedding:
    async def test_512_dim_vector_serializes(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="x",
        )
        session.add(upload)
        await session.commit()

        embedding = [0.1] * 512
        session.add(
            ParseResult(upload_id=upload.id, embedding=embedding, raw_text="t")
        )
        await session.commit()

        # 反序列化
        from sqlalchemy import select

        loaded = (
            await session.execute(
                select(ParseResult).where(ParseResult.upload_id == upload.id)
            )
        ).scalar_one()
        assert loaded.embedding is not None
        assert len(loaded.embedding) == 512


class TestVerifyResult:
    async def test_create_with_jsonb_fields(self, session: AsyncSession) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="x",
        )
        session.add(upload)
        await session.commit()

        v = VerifyResult(
            upload_id=upload.id,
            match_rate=85.50,
            checkpoints=[{"name": "abstract", "passed": True}],
            missing_items=["conclusion"],
            logic_issues=[],
            overall_confidence=78,
        )
        session.add(v)
        await session.commit()

        await session.refresh(v)
        assert v.checkpoints == [{"name": "abstract", "passed": True}]
        assert v.missing_items == ["conclusion"]


class TestCascadeDelete:
    async def test_delete_upload_cascades(self, session: AsyncSession) -> None:
        from sqlalchemy import select

        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="x.pdf",
            file_type="pdf",
            file_size=10,
            storage_path="x",
        )
        session.add(upload)
        await session.commit()
        session.add(ParseResult(upload_id=upload.id, raw_text="z"))
        session.add(VerifyResult(upload_id=upload.id, overall_confidence=80))
        await session.commit()
        upload_id = upload.id

        await session.delete(upload)
        await session.commit()

        prs = (
            await session.execute(
                select(ParseResult).where(ParseResult.upload_id == upload_id)
            )
        ).scalars().all()
        vrs = (
            await session.execute(
                select(VerifyResult).where(VerifyResult.upload_id == upload_id)
            )
        ).scalars().all()
        assert prs == []
        assert vrs == []
