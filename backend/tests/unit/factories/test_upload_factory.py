"""Task 8.6 验收：UploadFactory + sample 文件."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import BusinessRuleError
from app.utils.magic_check import assert_extension_matches
from tests.factories.upload_factory import SAMPLES, UploadFactory, get_sample


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


class TestUploadFactory:
    async def test_create_default(self, session: AsyncSession) -> None:
        u = await UploadFactory.create_async(session)
        assert u.id is not None
        assert u.parse_status == "parsed"
        assert u.file_type == "docx"


class TestSamples:
    def test_pdf_sample_recognized(self) -> None:
        ext = assert_extension_matches("sample.pdf", get_sample("sample.pdf"))
        assert ext == "pdf"

    def test_docx_sample_recognized(self) -> None:
        ext = assert_extension_matches("sample.docx", get_sample("sample.docx"))
        assert ext == "docx"

    def test_png_sample_recognized(self) -> None:
        ext = assert_extension_matches("sample.png", get_sample("sample.png"))
        assert ext == "png"

    def test_tampered_pdf_rejected(self) -> None:
        """tampered.pdf 实际是 png 改名 → magic check 应拒绝。"""
        with pytest.raises(BusinessRuleError):
            assert_extension_matches(
                "tampered.pdf", get_sample("tampered.pdf")
            )

    def test_unknown_sample_raises(self) -> None:
        with pytest.raises(KeyError):
            get_sample("nonexistent.xyz")

    def test_sample_inventory(self) -> None:
        # 至少 5 个 sample 类型可用
        assert len(SAMPLES) >= 5
