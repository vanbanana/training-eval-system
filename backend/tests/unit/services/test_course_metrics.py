"""Epic 19.4 验收：aggregate_course_metrics 课程级聚合."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.evaluation import DimensionScore, Evaluation
from app.models.upload import Upload
from app.services.profile_service import aggregate_course_metrics
from tests.factories.org_factory import CourseFactory
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


class TestAggregateCourseMetrics:
    async def test_empty_course_returns_zeros(
        self, session: AsyncSession
    ) -> None:
        course = await CourseFactory.create_async(session)
        await session.commit()
        m = await aggregate_course_metrics(session, course_id=course.id)
        assert m["total_evaluations"] == 0
        assert m["total_students"] == 0

    async def test_low_ratio_for_weak_dim(
        self, session: AsyncSession
    ) -> None:
        """Given 70% 学生在某维度 <60 When aggregate Then low_ratio ≥ 0.5."""
        course = await CourseFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(
            session, course_id=course.id, with_dimensions=2
        )
        await session.commit()
        # 7 学生低分，3 学生高分
        for i in range(10):
            student = await UserFactory.create_async(session)
            upload = Upload(
                task_id=task.id,
                student_id=student.id,
                filename="r",
                file_type="docx",
                file_size=10,
                storage_path=f"x{i}",
                parse_status="parsed",
            )
            session.add(upload)
            await session.flush()
            ev = Evaluation(
                task_id=task.id,
                student_id=student.id,
                upload_id=upload.id,
                status="auto_scored",
                total_score=50.0 if i < 7 else 90.0,
            )
            session.add(ev)
            await session.flush()
            for d in task.dimensions:
                session.add(
                    DimensionScore(
                        evaluation_id=ev.id,
                        dimension_id=d.id,
                        ai_score=50 if i < 7 else 90,
                        rationale="ok",
                    )
                )
        await session.commit()

        m = await aggregate_course_metrics(session, course_id=course.id)
        assert m["total_evaluations"] == 10
        assert m["total_students"] == 10
        assert any(d["low_ratio"] >= 0.5 for d in m["dimension_distributions"])
