"""Epic 20.4 验收：ReportService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import AuthorizationError
from app.models.evaluation import DimensionScore, Evaluation
from app.models.upload import Upload
from app.services.report_service import (
    NoEvaluationDataError,
    ReportService,
)
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import TeacherFactory, UserFactory


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


async def _seed(session: AsyncSession):
    teacher = await TeacherFactory.create_async(session)
    student = await UserFactory.create_async(session)
    task = await TrainingTaskFactory.create_async(
        session, teacher=teacher, with_dimensions=2
    )
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
        total_score=85.0,
    )
    session.add(ev)
    await session.flush()
    for d in task.dimensions:
        session.add(
            DimensionScore(
                evaluation_id=ev.id,
                dimension_id=d.id,
                ai_score=85,
                rationale="ok",
            )
        )
    await session.commit()
    return ev, student, task, teacher


class TestPersonalPdf:
    async def test_owner_can_download(self, session: AsyncSession) -> None:
        ev, student, _, _ = await _seed(session)
        svc = ReportService()
        pdf, name = await svc.generate_personal_pdf(
            session, evaluation_id=ev.id, operator=student
        )
        assert isinstance(pdf, bytes) and len(pdf) > 100
        assert name.endswith(".pdf")

    async def test_other_student_forbidden(
        self, session: AsyncSession
    ) -> None:
        ev, _, _, _ = await _seed(session)
        other = await UserFactory.create_async(session)
        await session.commit()
        svc = ReportService()
        with pytest.raises(AuthorizationError):
            await svc.generate_personal_pdf(
                session, evaluation_id=ev.id, operator=other
            )


class TestStatisticsXlsx:
    async def test_teacher_can_export(self, session: AsyncSession) -> None:
        ev, _, task, teacher = await _seed(session)
        svc = ReportService()
        xlsx, name = await svc.generate_statistics_xlsx(
            session, task_id=task.id, operator=teacher
        )
        assert isinstance(xlsx, bytes) and len(xlsx) > 100
        assert name.endswith(".xlsx")

    async def test_no_evaluation_raises(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(session, teacher=teacher)
        await session.commit()
        svc = ReportService()
        with pytest.raises(NoEvaluationDataError):
            await svc.generate_statistics_xlsx(
                session, task_id=task.id, operator=teacher
            )

    async def test_other_teacher_forbidden(
        self, session: AsyncSession
    ) -> None:
        _, _, task, _ = await _seed(session)
        other = await TeacherFactory.create_async(session)
        await session.commit()
        svc = ReportService()
        with pytest.raises(AuthorizationError):
            await svc.generate_statistics_xlsx(
                session, task_id=task.id, operator=other
            )
