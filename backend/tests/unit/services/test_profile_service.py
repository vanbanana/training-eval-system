"""Epic 18.3 验收：ProfileService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.evaluation import DimensionScore, Evaluation
from app.models.upload import Upload
from app.services.profile_service import (
    InsufficientDataError,
    ProfileService,
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


async def _seed_evals(
    session: AsyncSession, student, count: int = 3, score: int = 70
) -> None:
    for _ in range(count):
        task = await TrainingTaskFactory.create_async(session, with_dimensions=2)
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
        await session.flush()
        ev = Evaluation(
            task_id=task.id,
            student_id=student.id,
            upload_id=upload.id,
            status="auto_scored",
            total_score=float(score),
        )
        session.add(ev)
        await session.flush()
        for d in task.dimensions:
            session.add(
                DimensionScore(
                    evaluation_id=ev.id,
                    dimension_id=d.id,
                    ai_score=float(score),
                    rationale="ok",
                )
            )
    await session.commit()


class TestComputeStudentProfile:
    async def test_insufficient_data_raises(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        await _seed_evals(session, student, count=2)

        svc = ProfileService()
        with pytest.raises(InsufficientDataError):
            await svc.compute_student_profile(session, student_id=student.id)

    async def test_creates_profile_with_radar(
        self, session: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(session)
        await _seed_evals(session, student, count=3, score=75)

        svc = ProfileService()
        profile = await svc.compute_student_profile(session, student_id=student.id)
        await session.commit()

        assert profile.id is not None
        assert profile.radar_data is not None
        assert profile.source_evaluation_count == 3
        assert len(profile.score_trend or []) == 3

    async def test_upsert_idempotent(
        self, session: AsyncSession
    ) -> None:
        """重复触发同一学生 → 仅一行（UPSERT）."""
        student = await UserFactory.create_async(session)
        await _seed_evals(session, student, count=3)
        svc = ProfileService()
        p1 = await svc.compute_student_profile(session, student_id=student.id)
        await session.commit()
        first_id = p1.id
        p2 = await svc.compute_student_profile(session, student_id=student.id)
        await session.commit()
        assert p2.id == first_id
