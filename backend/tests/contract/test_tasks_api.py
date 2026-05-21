"""Task 5.5 验收：Task API 契约."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.tasks import router as tasks_router
from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from app.models.course import ClassMembership
from tests.factories.org_factory import ClassFactory, CourseFactory, MembershipFactory
from tests.factories.user_factory import (
    AdminFactory,
    TeacherFactory,
    UserFactory,
)


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
    app.include_router(tasks_router)

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


class TestCreateTask:
    async def test_teacher_creates_draft(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="t")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        r = await c.post(
            "/api/tasks",
            json={
                "name": "T1",
                "description": "测试",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
            },
            headers=_auth(token),
        )
        assert r.status_code == 201, r.text
        body = r.json()
        assert body["status"] == "draft"


class TestPublishFlow:
    async def test_full_flow_draft_to_published_to_student_visible(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tf")
        student = await UserFactory.create_async(db, username="sf")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        # 学生加入班级
        await MembershipFactory.create_async(db, class_obj=cls, student=student)
        await db.commit()

        teacher_token = create_access_token(user_id=teacher.id, role="teacher")
        student_token = create_access_token(user_id=student.id, role="student")

        # 1. 建草稿
        created = await c.post(
            "/api/tasks",
            json={
                "name": "Flow",
                "description": "",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
            },
            headers=_auth(teacher_token),
        )
        task_id = created.json()["id"]

        # 2. 设维度
        r2 = await c.put(
            f"/api/tasks/{task_id}/dimensions",
            json={
                "dimensions": [
                    {"name": "代码", "weight": 50},
                    {"name": "报告", "weight": 50},
                ]
            },
            headers=_auth(teacher_token),
        )
        assert r2.status_code == 200

        # 3. 学生此时看不到（draft）
        r_s_before = await c.get("/api/tasks", headers=_auth(student_token))
        assert all(t["id"] != task_id for t in r_s_before.json())

        # 4. 发布
        r3 = await c.post(
            f"/api/tasks/{task_id}/publish", headers=_auth(teacher_token)
        )
        assert r3.status_code == 200, r3.text
        assert r3.json()["status"] == "published"

        # 5. 学生现在能看到
        r_s_after = await c.get("/api/tasks", headers=_auth(student_token))
        ids = [t["id"] for t in r_s_after.json()]
        assert task_id in ids


class TestStudentVisibility:
    async def test_student_not_in_class_cannot_see(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        """Property 13：学生看不到不属于自己班级的任务。"""
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tv")
        student_in = await UserFactory.create_async(db, username="sv-in")
        student_out = await UserFactory.create_async(db, username="sv-out")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await MembershipFactory.create_async(db, class_obj=cls, student=student_in)
        await db.commit()

        teacher_token = create_access_token(user_id=teacher.id, role="teacher")

        created = await c.post(
            "/api/tasks",
            json={
                "name": "V",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
            },
            headers=_auth(teacher_token),
        )
        task_id = created.json()["id"]
        await c.put(
            f"/api/tasks/{task_id}/dimensions",
            json={
                "dimensions": [
                    {"name": "A", "weight": 50},
                    {"name": "B", "weight": 50},
                ]
            },
            headers=_auth(teacher_token),
        )
        await c.post(
            f"/api/tasks/{task_id}/publish", headers=_auth(teacher_token)
        )

        # 班内学生：可见
        in_token = create_access_token(user_id=student_in.id, role="student")
        r_in = await c.get("/api/tasks", headers=_auth(in_token))
        assert any(t["id"] == task_id for t in r_in.json())

        # 班外学生：不可见
        out_token = create_access_token(user_id=student_out.id, role="student")
        r_out = await c.get("/api/tasks", headers=_auth(out_token))
        assert all(t["id"] != task_id for t in r_out.json())


class TestDelete:
    async def test_delete_draft_succeeds(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="td")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        created = await c.post(
            "/api/tasks",
            json={"name": "D", "course_id": course.id, "class_ids": [cls.id]},
            headers=_auth(token),
        )
        task_id = created.json()["id"]

        r = await c.delete(f"/api/tasks/{task_id}", headers=_auth(token))
        assert r.status_code == 204

    async def test_delete_published_rejected(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tdp")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        created = await c.post(
            "/api/tasks",
            json={
                "name": "DP",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
            },
            headers=_auth(token),
        )
        task_id = created.json()["id"]
        await c.put(
            f"/api/tasks/{task_id}/dimensions",
            json={
                "dimensions": [
                    {"name": "A", "weight": 50},
                    {"name": "B", "weight": 50},
                ]
            },
            headers=_auth(token),
        )
        await c.post(f"/api/tasks/{task_id}/publish", headers=_auth(token))

        r = await c.delete(f"/api/tasks/{task_id}", headers=_auth(token))
        assert r.status_code == 400


class TestPublishValidation:
    async def test_set_dimensions_with_invalid_weights_returns_400(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        """Given 维度权重和 ≠ 100；When 调用 PUT dimensions；Then 400 + WEIGHT_SUM_INVALID."""
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tp")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        created = await c.post(
            "/api/tasks",
            json={
                "name": "P",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
            },
            headers=_auth(token),
        )
        task_id = created.json()["id"]

        r = await c.put(
            f"/api/tasks/{task_id}/dimensions",
            json={
                "dimensions": [
                    {"name": "A", "weight": 30},
                    {"name": "B", "weight": 30},  # 总和 60 ≠ 100
                ]
            },
            headers=_auth(token),
        )
        assert r.status_code == 400
        assert r.json()["error_code"] == "WEIGHT_SUM_INVALID"

    async def test_publish_without_dimensions_returns_400(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="tp2")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")

        created = await c.post(
            "/api/tasks",
            json={
                "name": "P2",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": (datetime.now(UTC) + timedelta(days=7)).isoformat(),
            },
            headers=_auth(token),
        )
        task_id = created.json()["id"]

        r = await c.post(f"/api/tasks/{task_id}/publish", headers=_auth(token))
        assert r.status_code == 400
        assert r.json()["error_code"] == "DIMENSION_COUNT_INVALID"
