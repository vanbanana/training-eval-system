"""Epic 21.6 验收：Notification API."""

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
from app.services.notification_events import TASK_PUBLISHED
from app.services.notification_service import NotificationService
from tests.factories.user_factory import UserFactory


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


class TestNotificationsApi:
    async def test_list_and_mark_read(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        u = await UserFactory.create_async(db)
        await db.commit()
        svc = NotificationService()
        await svc.send(
            db,
            recipient_ids=[u.id],
            event_type=TASK_PUBLISHED,
            title="x",
        )
        await db.commit()
        token = create_access_token(user_id=u.id, role="student")
        r = await client.get("/api/notifications", headers=_auth(token))
        assert r.status_code == 200
        body = r.json()
        assert body["unread_count"] == 1

        nid = body["items"][0]["id"]
        r2 = await client.post(
            f"/api/notifications/{nid}/read", headers=_auth(token)
        )
        assert r2.status_code == 200
        r3 = await client.get("/api/notifications", headers=_auth(token))
        assert r3.json()["unread_count"] == 0

    async def test_mark_other_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        u1 = await UserFactory.create_async(db)
        u2 = await UserFactory.create_async(db)
        await db.commit()
        svc = NotificationService()
        await svc.send(
            db,
            recipient_ids=[u1.id],
            event_type=TASK_PUBLISHED,
            title="x",
        )
        await db.commit()
        items = await svc.list_for_user(db, user_id=u1.id)
        token2 = create_access_token(user_id=u2.id, role="student")
        r = await client.post(
            f"/api/notifications/{items[0].id}/read", headers=_auth(token2)
        )
        assert r.status_code == 403

    async def test_preferences_get_put(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        u = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=u.id, role="student")
        r = await client.get(
            "/api/notifications/preferences", headers=_auth(token)
        )
        assert r.status_code == 200
        prefs = r.json()
        assert prefs[TASK_PUBLISHED] is True

        r2 = await client.put(
            "/api/notifications/preferences",
            headers=_auth(token),
            json={"event_type": TASK_PUBLISHED, "enabled": False},
        )
        assert r2.status_code == 200

        r3 = await client.get(
            "/api/notifications/preferences", headers=_auth(token)
        )
        assert r3.json()[TASK_PUBLISHED] is False
