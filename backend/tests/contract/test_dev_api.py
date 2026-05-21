"""Epic 26.1/26.2/26.3/26.4 验收：Dev 调试端点."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.main import app
from tests.factories.user_factory import UserFactory


pytestmark = [pytest.mark.contract]


def _dev_headers() -> dict[str, str]:
    return {"X-Dev-Token": get_settings().dev_token}


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


class TestDevTokenGuard:
    async def test_missing_token_403(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.post(
            "/api/_dev/seed", params={"scale": "small"}
        )
        assert r.status_code == 403

    async def test_wrong_token_403(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.post(
            "/api/_dev/seed",
            params={"scale": "small"},
            headers={"X-Dev-Token": "wrong"},
        )
        assert r.status_code == 403


class TestSeed:
    async def test_seed_small(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.post(
            "/api/_dev/seed",
            params={"scale": "small"},
            headers=_dev_headers(),
        )
        assert r.status_code == 200
        body = r.json()
        assert body["students"] == 20

    async def test_seed_unknown_scale_400(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.post(
            "/api/_dev/seed",
            params={"scale": "huge"},
            headers=_dev_headers(),
        )
        assert r.status_code == 400


class TestClock:
    async def test_freeze_advance_restore(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.post(
            "/api/_dev/clock/freeze",
            json={"time": "2030-01-01T00:00:00Z"},
            headers=_dev_headers(),
        )
        assert r.status_code == 200
        assert "2030-01-01" in r.json()["frozen_at"]

        r2 = await client.post(
            "/api/_dev/clock/advance",
            params={"seconds": 60},
            headers=_dev_headers(),
        )
        assert r2.status_code == 200
        assert "2030-01-01T00:01:00" in r2.json()["now"]

        r3 = await client.post(
            "/api/_dev/clock/restore", headers=_dev_headers()
        )
        assert r3.status_code == 200


class TestNotificationsInjection:
    async def test_inject_notification(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        u = await UserFactory.create_async(db)
        await db.commit()
        r = await client.post(
            f"/api/_dev/notifications/{u.id}/inject",
            json={"title": "测试", "content": "x"},
            headers=_dev_headers(),
        )
        assert r.status_code == 200


class TestStateView:
    async def test_unknown_entity_404(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.get(
            "/api/_dev/state/unknown/1", headers=_dev_headers()
        )
        assert r.status_code == 404


class TestHealthFull:
    async def test_returns_db_ok(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        r = await client.get(
            "/api/_dev/health/full", headers=_dev_headers()
        )
        assert r.status_code == 200
        body = r.json()
        assert body["db"] == "ok"
