"""Epic 15 验收：智能核查引擎."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.task import TrainingTask
from app.models.upload import ParseResult, Upload
from app.services.verify_engine import (
    VerifyEngine,
    extract_checkpoints,
    keyword_match_rate,
)
from tests.factories.task_factory import TrainingTaskFactory
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


class TestExtractCheckpoints:
    def test_numbered_list(self) -> None:
        text_in = "1. 完成源代码\n2. 提交报告\n3. 含测试用例"
        cps = extract_checkpoints(text_in)
        assert cps == ["完成源代码", "提交报告", "含测试用例"]

    def test_chinese_numbers(self) -> None:
        text_in = "一、报告\n二、源码\n三、测试"
        cps = extract_checkpoints(text_in)
        assert "报告" in cps

    def test_empty(self) -> None:
        assert extract_checkpoints("") == []


class TestKeywordMatch:
    def test_full_match(self) -> None:
        rate, missing = keyword_match_rate(
            "包含 报告 内容 还有 源码 部分",
            ["报告", "源码"],
        )
        assert rate == 100.0
        assert missing == []

    def test_partial_match(self) -> None:
        rate, missing = keyword_match_rate(
            "只有 报告",
            ["报告", "源码", "测试"],
        )
        assert 30 < rate < 40  # 1/3
        assert "源码" in missing
        assert "测试" in missing

    def test_no_checkpoints_returns_100(self) -> None:
        rate, missing = keyword_match_rate("any", [])
        assert rate == 100.0


class TestVerifyEngine:
    async def test_full_flow(self, session: AsyncSession) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        # 设 requirements
        task.requirements = "1. 完成源代码\n2. 提交实验报告\n3. 含测试用例"
        await session.commit()

        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r.docx",
            file_type="docx",
            file_size=100,
            storage_path="x",
            parse_status="parsed",
        )
        session.add(upload)
        await session.commit()
        session.add(
            ParseResult(
                upload_id=upload.id,
                raw_text="本提交包含完整的源代码和实验报告，包括 6 项测试用例，详细分析了性能。",
            )
        )
        await session.commit()

        engine = VerifyEngine()
        result = await engine.run(session, upload_id=upload.id)
        await session.commit()

        assert result.id is not None
        assert result.match_rate is not None
        assert result.match_rate > 0
        # missing_items 类型正确
        assert isinstance(result.missing_items, list)
        assert isinstance(result.checkpoints, list)
