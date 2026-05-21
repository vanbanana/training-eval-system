"""Task 4.4 验收：组织 API 契约."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.classes import router as classes_router
from app.api.courses import router as courses_router
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
    app.include_router(courses_router)
    app.include_router(classes_router)

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


async def _admin_token(db: AsyncSession) -> str:
    a = await AdminFactory.create_async(db, username="adm-c")
    await db.commit()
    return create_access_token(user_id=a.id, role="admin")


async def _teacher_token(db: AsyncSession) -> tuple[str, int]:
    t = await TeacherFactory.create_async(db, username="t-c")
    await db.commit()
    return create_access_token(user_id=t.id, role="teacher"), t.id


class TestCourseEndpoints:
    async def test_admin_can_create_course(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token = await _admin_token(db)
        r = await c.post(
            "/api/courses", json={"name": "DS", "code": "DS01"}, headers=_auth(token)
        )
        assert r.status_code == 201
        body = r.json()
        assert body["code"] == "DS01"

    async def test_teacher_create_course_returns_403(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token, _ = await _teacher_token(db)
        r = await c.post(
            "/api/courses", json={"name": "X", "code": "X01"}, headers=_auth(token)
        )
        assert r.status_code == 403

    async def test_archive_marks_archived(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        token = await _admin_token(db)
        created = (
            await c.post(
                "/api/courses",
                json={"name": "T", "code": "T-Arch"},
                headers=_auth(token),
            )
        ).json()
        course_id = created["id"]

        r = await c.patch(
            f"/api/courses/{course_id}/archive", headers=_auth(token)
        )
        assert r.status_code == 200
        assert r.json()["is_archived"] is True


class TestClassEndpoints:
    async def test_teacher_creates_class_for_self(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        admin_token = await _admin_token(db)
        teacher_token, teacher_id = await _teacher_token(db)

        # Admin 建课程
        course = (
            await c.post(
                "/api/courses",
                json={"name": "C", "code": "C-T"},
                headers=_auth(admin_token),
            )
        ).json()

        # 教师建班级
        r = await c.post(
            "/api/classes",
            json={"name": "K1", "course_id": course["id"]},
            headers=_auth(teacher_token),
        )
        assert r.status_code == 201
        body = r.json()
        assert body["teacher_id"] == teacher_id

    async def test_archived_course_cannot_create_class(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        admin_token = await _admin_token(db)
        teacher_token, _ = await _teacher_token(db)

        course = (
            await c.post(
                "/api/courses",
                json={"name": "ARK", "code": "C-Ark"},
                headers=_auth(admin_token),
            )
        ).json()
        await c.patch(
            f"/api/courses/{course['id']}/archive", headers=_auth(admin_token)
        )

        r = await c.post(
            "/api/classes",
            json={"name": "X", "course_id": course["id"]},
            headers=_auth(teacher_token),
        )
        assert r.status_code == 409


class TestBulkAddStudents:
    async def test_admin_adds_students_in_bulk(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        admin_token = await _admin_token(db)
        teacher_token, teacher_id = await _teacher_token(db)

        # 课程
        course = (
            await c.post(
                "/api/courses",
                json={"name": "C", "code": "C-Bk"},
                headers=_auth(admin_token),
            )
        ).json()
        # 班级
        cls = (
            await c.post(
                "/api/classes",
                json={"name": "B-1", "course_id": course["id"]},
                headers=_auth(teacher_token),
            )
        ).json()

        # 学生
        students = []
        for i in range(3):
            s = await UserFactory.create_async(db, username=f"bs{i}")
            students.append(s.id)
        await db.commit()

        r = await c.post(
            f"/api/classes/{cls['id']}/students/bulk",
            json={"student_ids": [*students, 99999]},
            headers=_auth(teacher_token),
        )
        assert r.status_code == 200
        body = r.json()
        assert len(body["added"]) == 3
        assert len(body["failed"]) == 1
        assert body["failed"][0]["student_id"] == 99999
