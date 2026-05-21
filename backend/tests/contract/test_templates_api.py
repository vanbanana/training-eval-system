"""Task 6.4 验收：Template API."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.templates import router as templates_router
from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.template_factory import EvaluationTemplateFactory
from tests.factories.user_factory import TeacherFactory


pytestmark = pytest.mark.contract


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


@pytest.fixture()
async def app_db() -> AsyncIterator[tuple[AsyncClient, AsyncSession]]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)

    async def _get_db() -> AsyncIterator[AsyncSession]:
        async with SessionLocal() as s:
            try:
                yield s
                await s.commit()
            except Exception:
                await s.rollback()
                raise

    app = FastAPI()
    register_exception_handlers(app)
    app.dependency_overrides[get_db] = _get_db
    app.include_router(templates_router)

    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        seed = SessionLocal()
        try:
            yield c, seed
        finally:
            await seed.close()
    await engine.dispose()


def _auth(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


class TestCreateTemplate:
    async def test_teacher_creates_private(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tt")
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        r = await c.post(
            "/api/templates",
            json={
                "name": "T1",
                "visibility": "private",
                "dimensions": [
                    {"name": "代码", "weight": 60},
                    {"name": "文档", "weight": 40},
                ],
            },
            headers=_auth(token),
        )
        assert r.status_code == 201
        body = r.json()
        assert body["name"] == "T1"
        assert len(body["items"]) == 2

    async def test_teacher_cannot_create_system(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="ts")
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        r = await c.post(
            "/api/templates",
            json={"name": "S", "visibility": "system"},
            headers=_auth(token),
        )
        assert r.status_code == 403


class TestDeleteTemplate:
    async def test_other_user_cannot_delete(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        ta = await TeacherFactory.create_async(db, username="td-a")
        tb = await TeacherFactory.create_async(db, username="td-b")
        await db.commit()
        tpl = await EvaluationTemplateFactory.create_async(db, owner=ta)
        await db.commit()

        token_b = create_access_token(user_id=tb.id, role="teacher")
        r = await c.delete(
            f"/api/templates/{tpl.id}", headers=_auth(token_b)
        )
        assert r.status_code == 403


class TestApplyTemplate:
    async def test_apply_to_draft_succeeds(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tap")
        await db.commit()
        tpl = await EvaluationTemplateFactory.create_async(
            db, owner=teacher, with_dimensions=3
        )
        task = await TrainingTaskFactory.create_async(
            db, teacher=teacher, status="draft", with_dimensions=None
        )
        await db.commit()

        token = create_access_token(user_id=teacher.id, role="teacher")
        r = await c.post(
            f"/api/templates/{tpl.id}/apply",
            json={"task_id": task.id},
            headers=_auth(token),
        )
        assert r.status_code == 200
        assert r.json()["applied_count"] == 3

    async def test_apply_to_published_returns_400(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tap2")
        await db.commit()
        tpl = await EvaluationTemplateFactory.create_async(db, owner=teacher)
        task = await TrainingTaskFactory.create_async(
            db, teacher=teacher, status="published", with_dimensions=2
        )
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        r = await c.post(
            f"/api/templates/{tpl.id}/apply",
            json={"task_id": task.id},
            headers=_auth(token),
        )
        assert r.status_code == 400
        assert r.json()["error_code"] == "BUSINESS_RULE_VIOLATED"
