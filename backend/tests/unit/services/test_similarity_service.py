"""Epic 17.1/17.3/17.5 验收：SimilarityService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.similarity import SimilarityRecord
from app.models.upload import ParseResult, Upload
from app.services.similarity_service import (
    SimilarityEngine,
    compute_simhash,
    find_similar_segments,
    hamming_distance,
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


class TestSimHash:
    """Epic 17.1."""

    def test_identical_distance_zero(self) -> None:
        """Given 完全相同文本 When 比 simhash Then 距离=0."""
        s = "本课程主要讨论分布式系统的架构与一致性问题，详述各类经典算法。"
        a = compute_simhash(s)
        b = compute_simhash(s)
        assert hamming_distance(a, b) == 0

    def test_very_different_distance_large(self) -> None:
        """Given 完全不同语义的两段文本 When 比 simhash Then 距离 > 8."""
        a = compute_simhash(
            "深度学习在自然语言处理领域取得了显著进展，特别是 transformer 架构。"
        )
        b = compute_simhash(
            "今天我去菜市场买了西红柿与黄瓜，准备回家做凉拌菜。"
        )
        assert hamming_distance(a, b) > 8


class TestFindSimilarSegments:
    """Epic 17.5."""

    def test_short_text_returns_empty(self) -> None:
        out = find_similar_segments("a" * 10, "a" * 10)
        assert out == []

    def test_finds_overlap(self) -> None:
        common = (
            "本提交完成了所有必要功能：用户登录、注册、密码加密、JWT 鉴权、"
            "会话过期与刷新机制，并通过完整测试。"
        )
        a = "前缀差异 " + common + " 后缀差异 A"
        b = "另一个前缀 " + common + " 另一个后缀 B"
        out = find_similar_segments(a, b)
        assert any(seg["ratio"] >= 0.9 for seg in out)


class TestSimilarityEngine:
    """Epic 17.3."""

    async def test_same_task_detects_pair(
        self, session: AsyncSession
    ) -> None:
        """Given 同 task 两份高度相似 upload When detect Then 写入 1 条 record."""
        student_a = await UserFactory.create_async(session)
        student_b = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()

        text_common = (
            "本提交实现了完整的 CRUD 操作、用户登录与权限校验、"
            "数据库迁移与单元测试覆盖；"
            "整体架构遵循三层分层与依赖注入原则；"
            "性能与安全性均经过压力测试。"
        )

        ua = Upload(
            task_id=task.id,
            student_id=student_a.id,
            filename="a.docx",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        ub = Upload(
            task_id=task.id,
            student_id=student_b.id,
            filename="b.docx",
            file_type="docx",
            file_size=10,
            storage_path="y",
            parse_status="parsed",
        )
        session.add_all([ua, ub])
        await session.flush()
        sim_a = compute_simhash(text_common)
        session.add_all(
            [
                ParseResult(
                    upload_id=ua.id,
                    raw_text=text_common,
                    simhash=sim_a,
                    embedding=[0.5] * 32,
                ),
                ParseResult(
                    upload_id=ub.id,
                    raw_text=text_common + " 后缀略有差异",
                    simhash=compute_simhash(text_common + " 后缀略有差异"),
                    embedding=[0.5] * 32,
                ),
            ]
        )
        await session.commit()

        engine = SimilarityEngine(hamming_threshold=10)
        records = await engine.detect_for_upload(session, upload_id=ub.id)
        await session.commit()
        assert len(records) >= 1
        # 唯一约束：upload_a_id < upload_b_id
        for r in records:
            assert r.upload_a_id < r.upload_b_id
            assert r.task_id == task.id

    async def test_cross_task_excluded(
        self, session: AsyncSession
    ) -> None:
        """Given 不同 task 的相似提交 When detect Then 不产生记录（Property 16）."""
        student = await UserFactory.create_async(session)
        task_a = await TrainingTaskFactory.create_async(session)
        task_b = await TrainingTaskFactory.create_async(session)
        await session.commit()

        text_common = "完全一样的提交内容用于测试 task 隔离" * 10

        ua = Upload(
            task_id=task_a.id,
            student_id=student.id,
            filename="a",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        ub = Upload(
            task_id=task_b.id,
            student_id=student.id,
            filename="b",
            file_type="docx",
            file_size=10,
            storage_path="y",
            parse_status="parsed",
        )
        session.add_all([ua, ub])
        await session.flush()
        sim = compute_simhash(text_common)
        session.add_all(
            [
                ParseResult(upload_id=ua.id, raw_text=text_common, simhash=sim),
                ParseResult(upload_id=ub.id, raw_text=text_common, simhash=sim),
            ]
        )
        await session.commit()

        engine = SimilarityEngine()
        records = await engine.detect_for_upload(session, upload_id=ub.id)
        assert records == []

    async def test_hamming_above_threshold_filtered(
        self, session: AsyncSession
    ) -> None:
        """Given hamming 距离超阈 When detect Then 不进入二阶段."""
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session)
        await session.commit()
        ua = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="a",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        ub = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="b",
            file_type="docx",
            file_size=10,
            storage_path="y",
            parse_status="parsed",
        )
        session.add_all([ua, ub])
        await session.flush()
        session.add_all(
            [
                ParseResult(
                    upload_id=ua.id,
                    raw_text="完全不同 A " * 30,
                    simhash=compute_simhash("完全不同 A " * 30),
                ),
                ParseResult(
                    upload_id=ub.id,
                    raw_text="毫无相关 B " * 30,
                    simhash=compute_simhash("毫无相关 B " * 30),
                ),
            ]
        )
        await session.commit()

        engine = SimilarityEngine(hamming_threshold=2)
        records = await engine.detect_for_upload(session, upload_id=ub.id)
        assert records == []
