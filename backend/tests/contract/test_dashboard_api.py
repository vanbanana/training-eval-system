"""Epic 24.2 验收：Dashboard API."""

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
from tests.factories.user_factory import AdminFactory, TeacherFactory, UserFactory


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


class TestDashboardApi:
    async def test_admin_dashboard(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        r = await client.get("/api/dashboard", headers=_auth(token))
        assert r.status_code == 200
        body = r.json()
        assert body["role"] == "admin"
        assert "user_count" in body

    async def test_teacher_dashboard_no_admin_fields(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        r = await client.get("/api/dashboard", headers=_auth(token))
        assert r.status_code == 200
        body = r.json()
        # teacher 不应看到 admin 独有字段
        assert body["role"] == "teacher"
        assert "user_count" not in body
        assert "monthly_active_students" not in body

    async def test_student_dashboard(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student.id, role="student")
        r = await client.get("/api/dashboard", headers=_auth(token))
        assert r.status_code == 200
        body = r.json()
        assert body["role"] == "student"
        assert "pending_tasks" in body
        # 不暴露其他用户隐私
        assert "user_count" not in body
