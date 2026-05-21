"""Epic 17.7 验收：Similarity 集成流程."""

from __future__ import annotations

import pytest
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.upload import ParseResult, Upload
from app.services.similarity_service import SimilarityEngine, compute_simhash
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory


pytestmark = pytest.mark.integration


class TestSimilarityFlow:
    async def test_three_uploads_only_AB_match(
        self, sqlite_session: AsyncSession
    ) -> None:
        """Given 3 份提交（A、B 高相似；C 不同）When 对每份跑 detect Then 仅 1 条 (A,B) 记录."""
        task = await TrainingTaskFactory.create_async(sqlite_session)
        sa = await UserFactory.create_async(sqlite_session)
        sb = await UserFactory.create_async(sqlite_session)
        sc = await UserFactory.create_async(sqlite_session)
        await sqlite_session.commit()

        text_ab = (
            "本提交完成了完整 CRUD 操作，包含登录、注册、JWT 鉴权与刷新；"
            "数据库迁移使用 Alembic；测试覆盖率 95% 以上；性能压测通过。"
        ) * 3
        text_c = (
            "我去菜市场买了番茄、黄瓜、土豆和大蒜；"
            "今天天气晴朗，适合做凉拌菜与西红柿炒鸡蛋。"
        ) * 3

        ua = Upload(
            task_id=task.id,
            student_id=sa.id,
            filename="a",
            file_type="docx",
            file_size=10,
            storage_path="a",
            parse_status="parsed",
        )
        ub = Upload(
            task_id=task.id,
            student_id=sb.id,
            filename="b",
            file_type="docx",
            file_size=10,
            storage_path="b",
            parse_status="parsed",
        )
        uc = Upload(
            task_id=task.id,
            student_id=sc.id,
            filename="c",
            file_type="docx",
            file_size=10,
            storage_path="c",
            parse_status="parsed",
        )
        sqlite_session.add_all([ua, ub, uc])
        await sqlite_session.flush()
        sqlite_session.add_all(
            [
                ParseResult(
                    upload_id=ua.id,
                    raw_text=text_ab,
                    simhash=compute_simhash(text_ab),
                ),
                ParseResult(
                    upload_id=ub.id,
                    raw_text=text_ab + "（轻微差异）",
                    simhash=compute_simhash(text_ab + "（轻微差异）"),
                ),
                ParseResult(
                    upload_id=uc.id,
                    raw_text=text_c,
                    simhash=compute_simhash(text_c),
                ),
            ]
        )
        await sqlite_session.commit()

        engine = SimilarityEngine(hamming_threshold=10)
        await engine.detect_for_upload(sqlite_session, upload_id=ub.id)
        await engine.detect_for_upload(sqlite_session, upload_id=uc.id)
        await sqlite_session.commit()

        from sqlalchemy import select

        from app.models.similarity import SimilarityRecord

        rows = list(
            (
                await sqlite_session.execute(select(SimilarityRecord))
            ).scalars()
        )
        assert len(rows) == 1
        assert {rows[0].upload_a_id, rows[0].upload_b_id} == {ua.id, ub.id}
