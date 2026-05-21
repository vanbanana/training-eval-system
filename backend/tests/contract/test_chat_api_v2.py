"""Epic 22.5 / 22.7 / 22.8 验收：Chat API 标准端点."""

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


class TestChatV2:
    async def test_create_session_and_post_message(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student.id, role="student")

        r1 = await client.post(
            "/api/chat/sessions",
            headers=_auth(token),
            json={"title": "x"},
        )
        assert r1.status_code == 201
        sid = r1.json()["id"]

        r2 = await client.post(
            f"/api/chat/sessions/{sid}/messages",
            headers=_auth(token),
            json={"content": "为什么我得了 80 分？"},
        )
        assert r2.status_code == 202
        body = r2.json()
        assert "message_id" in body and "ws_topic" in body

    async def test_quota_endpoint(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student.id, role="student")
        r = await client.get("/api/chat/quota", headers=_auth(token))
        assert r.status_code == 200
        body = r.json()
        assert "used" in body and "limit" in body

    async def test_delete_session(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        student = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student.id, role="student")
        r1 = await client.post(
            "/api/chat/sessions",
            headers=_auth(token),
            json={"title": "x"},
        )
        sid = r1.json()["id"]
        r2 = await client.delete(
            f"/api/chat/sessions/{sid}", headers=_auth(token)
        )
        assert r2.status_code == 204
