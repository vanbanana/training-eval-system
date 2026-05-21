"""Epic 25.4 验收：Imports API."""

from __future__ import annotations

import io
from collections.abc import AsyncIterator

import pytest
from httpx import ASGITransport, AsyncClient
from openpyxl import Workbook
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from app.main import app
from tests.factories.org_factory import ClassFactory
from tests.factories.user_factory import (
    AdminFactory,
    TeacherFactory,
    UserFactory,
)


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


def _make_user_xlsx() -> bytes:
    wb = Workbook()
    ws = wb.active
    ws.append(["username", "display_name", "role", "password"])
    ws.append(["u_a", "用户A", "student", "Pa$$w0rd2024"])
    ws.append(["u_b", "用户B", "teacher", "Pa$$w0rd2024"])
    buf = io.BytesIO()
    wb.save(buf)
    return buf.getvalue()


def _make_student_xlsx() -> bytes:
    wb = Workbook()
    ws = wb.active
    ws.append(["username"])
    ws.append(["alice"])
    buf = io.BytesIO()
    wb.save(buf)
    return buf.getvalue()


class TestImportApi:
    async def test_user_template_download(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        r = await client.get(
            "/api/imports/template/user.xlsx", headers=_auth(token)
        )
        assert r.status_code == 200

    async def test_admin_can_import_users(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        files = {"file": ("u.xlsx", _make_user_xlsx())}
        r = await client.post(
            "/api/imports/users", headers=_auth(token), files=files
        )
        assert r.status_code == 202
        body = r.json()
        assert body["total"] == 2
        assert body["success"] == 2

    async def test_teacher_cannot_import_users(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        files = {"file": ("u.xlsx", _make_user_xlsx())}
        r = await client.post(
            "/api/imports/users", headers=_auth(token), files=files
        )
        assert r.status_code == 403

    async def test_class_student_import(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher)
        await UserFactory.create_async(db, username="alice")
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        files = {"file": ("s.xlsx", _make_student_xlsx())}
        r = await client.post(
            f"/api/imports/classes/{cls.id}/students",
            headers=_auth(token),
            files=files,
        )
        assert r.status_code == 202
        assert r.json()["success"] == 1

    async def test_get_job_404(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        r = await client.get(
            "/api/imports/9999", headers=_auth(token)
        )
        assert r.status_code == 404
