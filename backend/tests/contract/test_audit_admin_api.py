"""Epic 23.5 验收：Audit admin API."""

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
from app.services.audit_service import AuditService
from tests.factories.user_factory import AdminFactory, TeacherFactory


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


class TestAuditApi:
    async def test_admin_can_query(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        svc = AuditService()
        await svc.emit(db, action="user.create", user_id=1)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        r = await client.get("/api/audit/logs", headers=_auth(token))
        assert r.status_code == 200
        assert len(r.json()["items"]) >= 1

    async def test_teacher_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        t = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=t.id, role="teacher")
        r = await client.get("/api/audit/logs", headers=_auth(token))
        assert r.status_code == 403

    async def test_admin_can_export_csv(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        svc = AuditService()
        await svc.emit(db, action="task.publish", user_id=1)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        r = await client.get("/api/audit/export", headers=_auth(token))
        assert r.status_code == 200
        assert "text/csv" in r.headers["content-type"]
