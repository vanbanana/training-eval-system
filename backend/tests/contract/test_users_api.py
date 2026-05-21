"""Task 3.7 验收：Users CRUD API 契约."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.users import router as users_router
from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from tests.factories.user_factory import AdminFactory, TeacherFactory, UserFactory


pytestmark = pytest.mark.contract


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


@pytest.fixture()
async def app_db() -> AsyncIterator[tuple[AsyncClient, AsyncSession]]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
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
    app.include_router(users_router)

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


async def _make_admin_token(db: AsyncSession) -> str:
    admin = await AdminFactory.create_async(db, username="admin1")
    await db.commit()
    return create_access_token(user_id=admin.id, role="admin")


async def _make_teacher_token(db: AsyncSession) -> str:
    t = await TeacherFactory.create_async(db, username="t1")
    await db.commit()
    return create_access_token(user_id=t.id, role="teacher")


class TestListUsers:
    async def test_admin_can_list(self, app_db: tuple[AsyncClient, AsyncSession]) -> None:
        c, db = app_db
        token = await _make_admin_token(db)
        r = await c.get("/api/users", headers=_auth(token))
        assert r.status_code == 200
        assert isinstance(r.json(), list)

    async def test_teacher_rejected(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token = await _make_teacher_token(db)
        r = await c.get("/api/users", headers=_auth(token))
        assert r.status_code == 403


class TestCreateUser:
    async def test_admin_can_create(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token = await _make_admin_token(db)

        r = await c.post(
            "/api/users",
            json={
                "username": "newone",
                "display_name": "New One",
                "role": "student",
                "password": "Pwd12345!",
            },
            headers=_auth(token),
        )
        assert r.status_code == 201
        assert r.json()["username"] == "newone"

    async def test_validation_error_on_short_password(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token = await _make_admin_token(db)

        r = await c.post(
            "/api/users",
            json={
                "username": "x",
                "display_name": "X",
                "role": "student",
                "password": "short",
            },
            headers=_auth(token),
        )
        assert r.status_code == 422

    async def test_duplicate_returns_409(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token = await _make_admin_token(db)
        await UserFactory.create_async(db, username="existing")
        await db.commit()

        r = await c.post(
            "/api/users",
            json={
                "username": "existing",
                "display_name": "X",
                "role": "student",
                "password": "Pwd12345!",
            },
            headers=_auth(token),
        )
        assert r.status_code == 409


class TestToggleActive:
    async def test_toggle_active(self, app_db: tuple[AsyncClient, AsyncSession]) -> None:
        c, db = app_db
        token = await _make_admin_token(db)
        target = await UserFactory.create_async(db, username="target")
        await db.commit()
        target_id = target.id

        r = await c.patch(
            f"/api/users/{target_id}/toggle-active", headers=_auth(token)
        )
        assert r.status_code == 200, r.text
        assert r.json()["is_active"] is False, r.json()

        # 再次翻转
        r2 = await c.patch(
            f"/api/users/{target_id}/toggle-active", headers=_auth(token)
        )
        assert r2.status_code == 200, r2.text
        assert r2.json()["is_active"] is True, r2.json()


class TestResetPassword:
    async def test_reset_returns_204(self, app_db: tuple[AsyncClient, AsyncSession]) -> None:
        c, db = app_db
        token = await _make_admin_token(db)
        target = await UserFactory.create_async(db, username="rp")
        await db.commit()

        r = await c.post(
            f"/api/users/{target.id}/reset-password",
            json={"new_password": "Brand-NewPwd!"},
            headers=_auth(token),
        )
        assert r.status_code == 204
