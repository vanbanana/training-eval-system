"""Epic 17.6 验收：Similarity API."""

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
from app.models.similarity import SimilarityRecord
from app.models.upload import ParseResult, Upload
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


async def _seed(db: AsyncSession) -> tuple[int, int, int]:
    teacher = await TeacherFactory.create_async(db)
    student_a = await UserFactory.create_async(db)
    student_b = await UserFactory.create_async(db)
    task = await TrainingTaskFactory.create_async(db, teacher=teacher)
    ua = Upload(
        task_id=task.id,
        student_id=student_a.id,
        filename="a",
        file_type="docx",
        file_size=10,
        storage_path="x",
        parse_status="parsed",
    )
    ub = Upload(
        task_id=task.id,
        student_id=student_b.id,
        filename="b",
        file_type="docx",
        file_size=10,
        storage_path="y",
        parse_status="parsed",
    )
    db.add_all([ua, ub])
    await db.flush()
    db.add_all(
        [
            ParseResult(upload_id=ua.id, raw_text="同样的提交内容用来测 segment" * 5),
            ParseResult(upload_id=ub.id, raw_text="同样的提交内容用来测 segment" * 5),
        ]
    )
    low, high = sorted([ua.id, ub.id])
    rec = SimilarityRecord(
        task_id=task.id,
        upload_a_id=low,
        upload_b_id=high,
        hamming_distance=2,
        cosine_similarity=0.95,
        state="suspect",
    )
    db.add(rec)
    await db.commit()
    return teacher.id, task.id, rec.id


class TestListSimilarity:
    async def test_student_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        _, task_id, _ = await _seed(db)
        student = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student.id, role="student")
        r = await client.get(
            f"/api/similarity/task/{task_id}", headers=_auth(token)
        )
        assert r.status_code == 403

    async def test_teacher_can_list(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher_id, task_id, rec_id = await _seed(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        r = await client.get(
            f"/api/similarity/task/{task_id}", headers=_auth(token)
        )
        assert r.status_code == 200
        assert any(item["id"] == rec_id for item in r.json())

    async def test_other_teacher_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        _, task_id, _ = await _seed(db)
        other = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=other.id, role="teacher")
        r = await client.get(
            f"/api/similarity/task/{task_id}", headers=_auth(token)
        )
        assert r.status_code == 403


class TestDecision:
    async def test_teacher_decide(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher_id, _, rec_id = await _seed(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        r = await client.post(
            f"/api/similarity/{rec_id}/decision",
            headers=_auth(token),
            json={"action": "confirm"},
        )
        assert r.status_code == 200
        assert r.json()["state"] == "confirmed"


class TestSegments:
    async def test_returns_segments(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher_id, _, rec_id = await _seed(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        r = await client.get(
            f"/api/similarity/{rec_id}/segments", headers=_auth(token)
        )
        assert r.status_code == 200
        assert isinstance(r.json(), list)
