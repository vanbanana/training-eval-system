"""Task 3.5 验收：当前用户依赖 + 角色守卫."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from fastapi import Depends, FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.deps import (
    AdminUser,
    CurrentUser,
    DbSession,
    TeacherUser,
    require_roles,
)
from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from tests.factories.user_factory import AdminFactory, TeacherFactory, UserFactory


pytestmark = pytest.mark.integration


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


@pytest.fixture()
async def app_and_db() -> AsyncIterator[tuple[FastAPI, AsyncSession]]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)

    async def _get_db_override() -> AsyncIterator[AsyncSession]:
        async with SessionLocal() as s:
            try:
                yield s
                await s.commit()
            except Exception:
                await s.rollback()
                raise

    app = FastAPI()
    app.dependency_overrides[get_db] = _get_db_override
    register_exception_handlers(app)

    @app.get("/me")
    async def me(user: CurrentUser) -> dict[str, object]:
        return {"id": user.id, "username": user.username, "role": user.role}

    @app.get("/admin-only")
    async def admin_only(_user: AdminUser) -> dict[str, str]:
        return {"ok": "admin"}

    @app.get("/teacher-or-admin")
    async def teacher(_user: TeacherUser) -> dict[str, str]:
        return {"ok": "teacher"}

    seed_session = SessionLocal()
    try:
        yield app, seed_session
    finally:
        await seed_session.close()
        await engine.dispose()


async def test_no_token_returns_401(app_and_db: tuple[FastAPI, AsyncSession]) -> None:
    app, _ = app_and_db
    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        r = await c.get("/me")
    assert r.status_code == 401


async def test_invalid_token_returns_401(app_and_db: tuple[FastAPI, AsyncSession]) -> None:
    app, _ = app_and_db
    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        r = await c.get("/me", headers={"Authorization": "Bearer invalid"})
    assert r.status_code == 401


async def test_valid_token_returns_user(app_and_db: tuple[FastAPI, AsyncSession]) -> None:
    app, db = app_and_db
    user = await UserFactory.create_async(db, username="alice")
    await db.commit()

    token = create_access_token(user_id=user.id, role=user.role)
    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        r = await c.get("/me", headers={"Authorization": f"Bearer {token}"})
    assert r.status_code == 200
    body = r.json()
    assert body["username"] == "alice"
    assert body["role"] == "student"


async def test_disabled_user_rejected(app_and_db: tuple[FastAPI, AsyncSession]) -> None:
    app, db = app_and_db
    user = await UserFactory.create_async(db, username="dis", is_active=False)
    await db.commit()

    token = create_access_token(user_id=user.id, role=user.role)
    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        r = await c.get("/me", headers={"Authorization": f"Bearer {token}"})
    assert r.status_code == 401


class TestRoleGuards:
    async def test_admin_only_rejects_student(
        self, app_and_db: tuple[FastAPI, AsyncSession]
    ) -> None:
        app, db = app_and_db
        student = await UserFactory.create_async(db, username="s")
        await db.commit()

        token = create_access_token(user_id=student.id, role=student.role)
        transport = ASGITransport(app=app, raise_app_exceptions=False)
        async with AsyncClient(transport=transport, base_url="http://t") as c:
            r = await c.get("/admin-only", headers={"Authorization": f"Bearer {token}"})
        assert r.status_code == 403

    async def test_admin_only_accepts_admin(
        self, app_and_db: tuple[FastAPI, AsyncSession]
    ) -> None:
        app, db = app_and_db
        admin = await AdminFactory.create_async(db, username="a")
        await db.commit()

        token = create_access_token(user_id=admin.id, role=admin.role)
        transport = ASGITransport(app=app, raise_app_exceptions=False)
        async with AsyncClient(transport=transport, base_url="http://t") as c:
            r = await c.get("/admin-only", headers={"Authorization": f"Bearer {token}"})
        assert r.status_code == 200

    async def test_teacher_role_allows_admin_too(
        self, app_and_db: tuple[FastAPI, AsyncSession]
    ) -> None:
        app, db = app_and_db
        admin = await AdminFactory.create_async(db, username="a2")
        await db.commit()

        token = create_access_token(user_id=admin.id, role=admin.role)
        transport = ASGITransport(app=app, raise_app_exceptions=False)
        async with AsyncClient(transport=transport, base_url="http://t") as c:
            r = await c.get(
                "/teacher-or-admin", headers={"Authorization": f"Bearer {token}"}
            )
        assert r.status_code == 200

    async def test_teacher_role_rejects_student(
        self, app_and_db: tuple[FastAPI, AsyncSession]
    ) -> None:
        app, db = app_and_db
        student = await UserFactory.create_async(db, username="s2")
        await db.commit()

        token = create_access_token(user_id=student.id, role=student.role)
        transport = ASGITransport(app=app, raise_app_exceptions=False)
        async with AsyncClient(transport=transport, base_url="http://t") as c:
            r = await c.get(
                "/teacher-or-admin", headers={"Authorization": f"Bearer {token}"}
            )
        assert r.status_code == 403
