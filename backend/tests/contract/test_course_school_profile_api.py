"""Epic 19.5 验收：教学画像 API."""

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
from tests.factories.org_factory import CourseFactory
from tests.factories.task_factory import TrainingTaskFactory
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


class TestCourseProfile:
    async def test_student_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        course = await CourseFactory.create_async(db)
        student = await UserFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=student.id, role="student")
        r = await client.get(
            f"/api/profiles/course/{course.id}", headers=_auth(token)
        )
        assert r.status_code == 403

    async def test_teacher_other_course_forbidden(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        course = await CourseFactory.create_async(db)
        other = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=other.id, role="teacher")
        r = await client.get(
            f"/api/profiles/course/{course.id}", headers=_auth(token)
        )
        assert r.status_code == 403

    async def test_teacher_own_course_allowed(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(db)
        course = await CourseFactory.create_async(db)
        await TrainingTaskFactory.create_async(
            db, teacher=teacher, course_id=course.id
        )
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        r = await client.get(
            f"/api/profiles/course/{course.id}?range=1m",
            headers=_auth(token),
        )
        assert r.status_code == 200
        assert "metrics" in r.json()

    async def test_admin_can_see_school(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=admin.id, role="admin")
        r = await client.get("/api/profiles/school", headers=_auth(token))
        assert r.status_code == 200

    async def test_teacher_cannot_see_school(
        self, client: AsyncClient, db: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(db)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        r = await client.get("/api/profiles/school", headers=_auth(token))
        assert r.status_code == 403
