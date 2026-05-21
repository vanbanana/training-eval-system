"""Epic 18.5 验收：Profile API."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from app.main import app
from app.models.evaluation import DimensionScore, Evaluation
from app.models.upload import Upload
from tests.factories.org_factory import ClassFactory, MembershipFactory
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import TeacherFactory, UserFactory


pytestmark = [pytest.mark.contract]


def _auth(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


@pytest.fixture()
async def db() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


@pytest.fixture()
async def client(db: AsyncSession) -> AsyncIterator[AsyncClient]:
    async def _override():
        yield db

    app.dependency_overrides[get_db] = _override
    register_exception_handlers(app)
    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        yield c
    app.dependency_overrides.pop(get_db, None)


async def _seed_with_evals(
    db: AsyncSession, count: int = 3, score: int = 75
):
    student = await UserFactory.create_async(db)
    for _ in range(count):
        task = await TrainingTaskFactory.create_async(db, with_dimensions=2)
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        db.add(upload)
        await db.flush()
        ev = Evaluation(
            task_id=task.id,
            student_id=student.id,
            upload_id=upload.id,
            status="auto_scored",
            total_score=float(score),
        )
        db.add(ev)
        await db.flush()
        for d in task.dimensions:
            db.add(
                DimensionScore(
                    evaluation_id=ev.id,
                    dimension_id=d.id,
                    ai_score=float(score),
                    rationale="ok",
                )
            )
    await db.commit()
    return student


class TestProfilesApi:
    async def test_self_can_read(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await _seed_with_evals(db, count=3)
        token = create_access_token(user_id=student.id, role="student")
        r = await client.get(
            f"/api/profiles/student/{student.id}", headers=_auth(token)
        )
        assert r.status_code == 200
        body = r.json()
        assert body["source_evaluation_count"] == 3

    async def test_other_student_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student_a = await _seed_with_evals(db, count=3)
        student_b = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student_b.id, role="student")
        r = await client.get(
            f"/api/profiles/student/{student_a.id}", headers=_auth(token)
        )
        assert r.status_code == 403

    async def test_insufficient_data_returns_flag(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await _seed_with_evals(db, count=2)
        token = create_access_token(user_id=student.id, role="student")
        r = await client.get(
            f"/api/profiles/student/{student.id}", headers=_auth(token)
        )
        assert r.status_code == 200
        assert r.json()["insufficient_data"] is True

    async def test_teacher_other_class_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await _seed_with_evals(db, count=3)
        teacher = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        r = await client.get(
            f"/api/profiles/student/{student.id}", headers=_auth(token)
        )
        assert r.status_code == 403

    async def test_teacher_own_class_allowed(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await _seed_with_evals(db, count=3)
        teacher = await TeacherFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher)
        await MembershipFactory.create_async(db, class_obj=cls, student=student)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        r = await client.get(
            f"/api/profiles/student/{student.id}", headers=_auth(token)
        )
        assert r.status_code == 200
