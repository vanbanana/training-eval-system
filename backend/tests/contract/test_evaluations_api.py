"""Epic 16.9 验收：Evaluation API 端点契约测试."""

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


async def _seed_eval(
    db: AsyncSession, *, teacher_id: int | None = None
) -> tuple[Evaluation, int, int]:
    teacher = (
        await TeacherFactory.create_async(db)
        if teacher_id is None
        else None
    )
    student = await UserFactory.create_async(db)
    task = await TrainingTaskFactory.create_async(
        db, teacher=teacher, with_dimensions=2
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
    db.add(upload)
    await db.flush()
    ev = Evaluation(
        task_id=task.id,
        student_id=student.id,
        upload_id=upload.id,
        status="auto_scored",
        total_score=80.0,
    )
    db.add(ev)
    await db.flush()
    # 显式查询 dimensions，避免 lazy load
    from sqlalchemy import select

    from app.models.task import Dimension

    dims = list(
        (
            await db.execute(
                select(Dimension).where(Dimension.task_id == task.id)
            )
        ).scalars()
    )
    for d in dims:
        db.add(
            DimensionScore(
                evaluation_id=ev.id,
                dimension_id=d.id,
                ai_score=80,
                rationale=("理由 " * 30),
            )
        )
    await db.commit()
    # 重新加载 ev 含 scores
    from sqlalchemy.orm import selectinload

    ev = (
        await db.execute(
            select(Evaluation)
            .options(selectinload(Evaluation.scores))
            .where(Evaluation.id == ev.id)
        )
    ).scalar_one()
    return ev, student.id, (teacher.id if teacher else (teacher_id or 0))


class TestGetEvaluation:
    async def test_owner_student_can_read(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        ev, student_id, _ = await _seed_eval(db)
        token = create_access_token(user_id=student_id, role="student")
        r = await client.get(f"/api/evaluations/{ev.id}", headers=_auth(token))
        assert r.status_code == 200
        assert r.json()["id"] == ev.id

    async def test_other_student_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        ev, _, _ = await _seed_eval(db)
        other = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=other.id, role="student")
        r = await client.get(f"/api/evaluations/{ev.id}", headers=_auth(token))
        assert r.status_code == 403


class TestUpdateDimension:
    async def test_teacher_update_subj(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        ev, _, teacher_id = await _seed_eval(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        first_dim = ev.scores[0].dimension_id
        r = await client.patch(
            f"/api/evaluations/{ev.id}/dimensions/{first_dim}",
            headers=_auth(token),
            json={"subj_score": 95, "comment": "棒"},
        )
        assert r.status_code == 200
        out = r.json()
        assert any(
            s["dimension_id"] == first_dim and s["teacher_score"] == 95
            for s in out["scores"]
        )


class TestBulkAction:
    async def test_reject_without_reason(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        ev, _, teacher_id = await _seed_eval(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        r = await client.post(
            "/api/evaluations/bulk-action",
            headers=_auth(token),
            json={
                "evaluation_ids": [ev.id],
                "action": "reject",
                "reason": "",
            },
        )
        assert r.status_code == 400


class TestHistory:
    async def test_teacher_can_view(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        ev, _, teacher_id = await _seed_eval(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        r = await client.get(
            f"/api/evaluations/{ev.id}/history", headers=_auth(token)
        )
        assert r.status_code == 200
        assert isinstance(r.json(), list)


class TestListForTask:
    async def test_teacher_can_list(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        ev, _, teacher_id = await _seed_eval(db)
        token = create_access_token(user_id=teacher_id, role="teacher")
        r = await client.get(
            f"/api/evaluations/by-task/{ev.task_id}", headers=_auth(token)
        )
        assert r.status_code == 200
        assert any(item["id"] == ev.id for item in r.json())
