"""Epic 16 验收：评价引擎 + Property 1/2."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import (
    BusinessRuleError,
    WeightSumInvalidError,
)
from app.models.evaluation import DimensionScore
from app.models.upload import Upload
from app.services.score_engine import ScoreEngine, compute_total_score
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


class TestComputeTotalScore:
    """Property 1: 综合分 = Σ(权重 × 维度分) / 100; 权重和必须 = 100."""

    def test_pure_ai_scores(self) -> None:
        scores = [
            DimensionScore(dimension_id=1, ai_score=80),
            DimensionScore(dimension_id=2, ai_score=90),
        ]
        dim_map = {1: 60, 2: 40}
        total = compute_total_score(scores, dim_map)
        # 80*0.6 + 90*0.4 = 48 + 36 = 84
        assert total == 84.0

    def test_teacher_overrides_ai(self) -> None:
        scores = [
            DimensionScore(dimension_id=1, ai_score=50, teacher_score=90),
            DimensionScore(dimension_id=2, ai_score=80),
        ]
        dim_map = {1: 50, 2: 50}
        # 90*0.5 + 80*0.5 = 85
        assert compute_total_score(scores, dim_map) == 85.0

    def test_clamped_to_100(self) -> None:
        """Property 2: 综合分 ∈ [0, 100]."""
        scores = [DimensionScore(dimension_id=1, ai_score=200)]  # 异常分
        dim_map = {1: 100}
        total = compute_total_score(scores, dim_map)
        assert total == 100.0

    def test_clamped_to_0(self) -> None:
        scores = [DimensionScore(dimension_id=1, ai_score=-50)]
        dim_map = {1: 100}
        total = compute_total_score(scores, dim_map)
        assert total == 0.0

    def test_weight_sum_invalid(self) -> None:
        scores = [DimensionScore(dimension_id=1, ai_score=80)]
        dim_map = {1: 50}  # 总和 50 ≠ 100
        with pytest.raises(WeightSumInvalidError):
            compute_total_score(scores, dim_map)


class TestScoreUpload:
    async def test_score_creates_evaluation(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(
            session, with_dimensions=2
        )
        await session.commit()

        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r.docx",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        session.add(upload)
        await session.commit()

        engine = ScoreEngine()
        ev = await engine.score_upload(session, upload_id=upload.id)
        await session.commit()

        assert ev.id is not None
        assert ev.status == "scored"
        assert ev.total_score is not None
        assert 0 <= ev.total_score <= 100

    async def test_duplicate_score_rejected(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(
            session, with_dimensions=2
        )
        await session.commit()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        session.add(upload)
        await session.commit()

        engine = ScoreEngine()
        await engine.score_upload(session, upload_id=upload.id)
        await session.commit()

        with pytest.raises(BusinessRuleError):
            await engine.score_upload(session, upload_id=upload.id)


class TestConfirm:
    async def test_teacher_override_recomputes(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(
            session, with_dimensions=2
        )
        await session.commit()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        session.add(upload)
        await session.commit()

        engine = ScoreEngine()
        ev = await engine.score_upload(session, upload_id=upload.id)
        await session.commit()
        original_total = ev.total_score

        # 教师覆盖：让所有维度都 100 分
        overrides = {ds.dimension_id: 100.0 for ds in ev.scores}
        confirmed = await engine.confirm(
            session,
            evaluation_id=ev.id,
            teacher_comment="完美",
            score_overrides=overrides,
        )
        await session.commit()
        assert confirmed.status == "confirmed"
        assert confirmed.total_score == 100.0
        assert confirmed.total_score != original_total
