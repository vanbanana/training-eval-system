"""Epic 15.6 验收：核查端到端集成测试."""

from __future__ import annotations

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.upload import ParseResult, Upload, VerifyResult
from app.services.verify_engine import VerifyEngine
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory


pytestmark = pytest.mark.integration


class TestVerifyFlowE2E:
    async def test_full_coverage_flow(
        self, sqlite_session: AsyncSession
    ) -> None:
        """Given 提交完全覆盖要求 When VerifyEngine.run Then VerifyResult 写入且 match_rate≥0.5."""
        student = await UserFactory.create_async(sqlite_session)
        task = await TrainingTaskFactory.create_async(sqlite_session)
        task.requirements = "1. 完成源代码\n2. 提交实验报告\n3. 含测试用例"
        await sqlite_session.commit()

        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r.docx",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        sqlite_session.add(upload)
        await sqlite_session.commit()
        sqlite_session.add(
            ParseResult(
                upload_id=upload.id,
                raw_text="本提交完成源代码、提交实验报告、并含完整测试用例",
            )
        )
        await sqlite_session.commit()

        engine = VerifyEngine()
        result = await engine.run(sqlite_session, upload_id=upload.id)
        await sqlite_session.commit()

        # DB 持久化校验
        loaded = await sqlite_session.get(VerifyResult, result.id)
        assert loaded is not None
        assert loaded.upload_id == upload.id
        assert loaded.match_rate is not None
        assert float(loaded.match_rate) >= 50.0

    async def test_partial_coverage_lists_missing(
        self, sqlite_session: AsyncSession
    ) -> None:
        """Given 部分缺失 When run Then missing_items 非空."""
        student = await UserFactory.create_async(sqlite_session)
        task = await TrainingTaskFactory.create_async(sqlite_session)
        task.requirements = "1. 用户登录功能\n2. 数据库设计\n3. 单元测试用例"
        await sqlite_session.commit()

        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r.docx",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        sqlite_session.add(upload)
        await sqlite_session.commit()
        sqlite_session.add(
            ParseResult(
                upload_id=upload.id,
                raw_text="实现了用户登录功能",
            )
        )
        await sqlite_session.commit()

        engine = VerifyEngine()
        result = await engine.run(sqlite_session, upload_id=upload.id)
        assert result.missing_items
        assert len(result.missing_items) >= 1
