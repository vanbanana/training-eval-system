"""Task 3.6 验收：Auth API 端点契约."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.auth import router as auth_router
from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from tests.factories.user_factory import UserFactory


pytestmark = pytest.mark.contract


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


@pytest.fixture()
async def client_and_db() -> AsyncIterator[tuple[AsyncClient, AsyncSession]]:
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
    app.include_router(auth_router)

    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        seed = SessionLocal()
        try:
            yield c, seed
        finally:
            await seed.close()
    await engine.dispose()


class TestLoginEndpoint:
    async def test_valid_login_returns_200_with_tokens(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = client_and_db
        await UserFactory.create_async(db, username="alice", password="StrongPwd123!")
        await db.commit()

        r = await c.post(
            "/api/auth/login",
            json={"username": "alice", "password": "StrongPwd123!"},
        )
        assert r.status_code == 200
        body = r.json()
        assert body["access_token"]
        assert body["refresh_token"]
        assert body["token_type"] == "bearer"
        assert body["user"]["username"] == "alice"
        assert body["user"]["role"] == "student"

    async def test_invalid_login_returns_401(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = client_and_db
        await UserFactory.create_async(db, username="bob", password="Right!")
        await db.commit()

        r = await c.post(
            "/api/auth/login", json={"username": "bob", "password": "Wrong!"}
        )
        assert r.status_code == 401
        body = r.json()
        assert body["error_code"] == "INVALID_CREDENTIALS"

    async def test_unknown_user_returns_401(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, _ = client_and_db
        r = await c.post(
            "/api/auth/login", json={"username": "ghost", "password": "x"}
        )
        assert r.status_code == 401

    async def test_missing_fields_returns_422(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, _ = client_and_db
        r = await c.post("/api/auth/login", json={"username": "x"})
        assert r.status_code == 422
        body = r.json()
        assert body["error_code"] == "VALIDATION_FAILED"


class TestRefreshEndpoint:
    async def test_valid_refresh_returns_new_tokens(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = client_and_db
        await UserFactory.create_async(db, username="rusr", password="Pwd123!")
        await db.commit()

        login_resp = await c.post(
            "/api/auth/login", json={"username": "rusr", "password": "Pwd123!"}
        )
        refresh = login_resp.json()["refresh_token"]

        r = await c.post("/api/auth/refresh", json={"refresh_token": refresh})
        assert r.status_code == 200
        body = r.json()
        assert body["access_token"]
        assert body["refresh_token"]

    async def test_missing_refresh_returns_422(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, _ = client_and_db
        r = await c.post("/api/auth/refresh", json={})
        assert r.status_code == 422


class TestMeEndpoint:
    async def test_authenticated_returns_user(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = client_and_db
        await UserFactory.create_async(db, username="me-user", password="Pwd123!")
        await db.commit()

        login_resp = await c.post(
            "/api/auth/login", json={"username": "me-user", "password": "Pwd123!"}
        )
        token = login_resp.json()["access_token"]

        r = await c.get("/api/auth/me", headers={"Authorization": f"Bearer {token}"})
        assert r.status_code == 200
        assert r.json()["username"] == "me-user"

    async def test_no_token_returns_401(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, _ = client_and_db
        r = await c.get("/api/auth/me")
        assert r.status_code == 401


class TestLogoutEndpoint:
    async def test_logout_returns_204(
        self, client_and_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, _ = client_and_db
        r = await c.post("/api/auth/logout")
        assert r.status_code == 204
