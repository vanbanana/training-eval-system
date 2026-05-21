"""Epic 14 验收：Parse Pipeline."""

from __future__ import annotations

import io
from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.services.parse_pipeline import ParsePipeline, compute_simhash
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory
from tests.fakes.fake_llm import FakeLLM
from tests.fakes.fake_storage import InMemoryStorage


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


def _make_docx_bytes(text_content: str) -> bytes:
    from docx import Document

    d = Document()
    d.add_paragraph(text_content)
    buf = io.BytesIO()
    d.save(buf)
    return buf.getvalue()


class TestSimHash:
    def test_same_text_same_hash(self) -> None:
        h1 = compute_simhash("hello world")
        h2 = compute_simhash("hello world")
        assert h1 == h2

    def test_different_text_different_hash(self) -> None:
        h1 = compute_simhash("hello world")
        h2 = compute_simhash("totally different content")
        assert h1 != h2

    def test_empty_returns_zero(self) -> None:
        assert compute_simhash("") == 0

    def test_64_bit_range(self) -> None:
        h = compute_simhash("test text content")
        # signed 64-bit
        assert -(1 << 63) <= h < (1 << 63)


class TestPipeline:
    async def test_full_parse_flow(self, session: AsyncSession) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        # 创建 upload + storage 内容
        from app.models.upload import Upload

        content = _make_docx_bytes("学生提交的实验报告内容")
        storage = InMemoryStorage()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r.docx",
            file_type="docx",
            file_size=len(content),
            storage_path="task_x/student_y/abc.docx",
            sha256="x" * 64,
            parse_status="pending",
        )
        session.add(upload)
        await session.commit()
        await storage.save(upload.storage_path, content)

        pipeline = ParsePipeline(storage=storage, llm=FakeLLM())
        result = await pipeline.run(session, upload_id=upload.id)
        await session.commit()

        assert result.id is not None
        assert "学生提交" in result.raw_text
        assert result.simhash != 0
        assert result.embedding is not None
        assert len(result.embedding) == 512

        # 状态已推进
        await session.refresh(upload)
        assert upload.parse_status == "parsed"

    async def test_parse_failure_marks_failed(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        from app.models.upload import Upload

        storage = InMemoryStorage()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="bad.docx",
            file_type="docx",
            file_size=10,
            storage_path="bad",
            parse_status="pending",
        )
        session.add(upload)
        await session.commit()
        # 写入非法 docx 内容
        await storage.save(upload.storage_path, b"not a docx")

        pipeline = ParsePipeline(storage=storage)
        with pytest.raises(Exception):  # noqa: B017
            await pipeline.run(session, upload_id=upload.id)
        await session.commit()

        await session.refresh(upload)
        assert upload.parse_status == "failed"
