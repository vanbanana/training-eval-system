"""Task 8.5 验收：Upload API 契约."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta
from pathlib import Path

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.api.uploads import router as uploads_router
from app.core.config import get_settings
from app.core.database import Base, get_db
from app.core.exception_handlers import register_exception_handlers
from app.core.security import create_access_token
from tests.factories.org_factory import (
    ClassFactory,
    CourseFactory,
    MembershipFactory,
)
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import TeacherFactory, UserFactory


pytestmark = pytest.mark.contract

_PDF = b"%PDF-1.4\n" + b"x" * 2000


@pytest.fixture(autouse=True)
def _set_jwt(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    monkeypatch.setenv("TES_UPLOAD_ROOT", str(tmp_path / "uploads"))
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
    app.include_router(uploads_router)

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


async def _setup(db: AsyncSession) -> tuple[int, int, int]:
    """返回 (student_id, task_id, student_token)."""
    teacher = await TeacherFactory.create_async(db, username="up-t")
    course = await CourseFactory.create_async(db)
    cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
    student = await UserFactory.create_async(db, username="up-s")
    await db.commit()
    await MembershipFactory.create_async(db, class_obj=cls, student=student)
    await db.commit()
    task = await TrainingTaskFactory.create_async(
        db,
        teacher=teacher,
        course_id=course.id,
        classes=[cls],
        deadline=datetime.now(UTC) + timedelta(days=7),
        with_dimensions=2,
        status="published",
    )
    await db.commit()
    return student.id, task.id, create_access_token(
        user_id=student.id, role="student"
    )


class TestUploadEndpoint:
    async def test_upload_succeeds(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        student_id, task_id, token = await _setup(db)

        files = {"file": ("report.pdf", _PDF, "application/pdf")}
        r = await c.post(
            f"/api/uploads/{task_id}", files=files, headers=_auth(token)
        )
        assert r.status_code == 201, r.text
        body = r.json()
        assert body["filename"] == "report.pdf"
        assert body["file_type"] == "pdf"
        assert body["parse_status"] == "pending"

    async def test_unauthenticated_returns_401(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, _ = app_db
        files = {"file": ("x.pdf", _PDF)}
        r = await c.post("/api/uploads/1", files=files)
        assert r.status_code == 401

    async def test_teacher_cannot_upload(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        teacher = await TeacherFactory.create_async(db, username="t-no")
        course = await CourseFactory.create_async(db)
        cls = await ClassFactory.create_async(db, teacher=teacher, course=course)
        await db.commit()
        task = await TrainingTaskFactory.create_async(
            db,
            teacher=teacher,
            course_id=course.id,
            classes=[cls],
            deadline=datetime.now(UTC) + timedelta(days=7),
            with_dimensions=2,
            status="published",
        )
        await db.commit()
        token = create_access_token(user_id=teacher.id, role="teacher")
        files = {"file": ("x.pdf", _PDF)}
        r = await c.post(
            f"/api/uploads/{task.id}", files=files, headers=_auth(token)
        )
        assert r.status_code == 403


class TestListUploads:
    async def test_returns_only_own(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        _, task_id, token = await _setup(db)

        # 上传 1 次
        files = {"file": ("r.pdf", _PDF)}
        await c.post(
            f"/api/uploads/{task_id}", files=files, headers=_auth(token)
        )

        r = await c.get(f"/api/uploads/{task_id}", headers=_auth(token))
        assert r.status_code == 200
        body = r.json()
        assert len(body) == 1


class TestDelete:
    async def test_student_can_delete_own(
        self, app_db: tuple[AsyncClient, AsyncSession]
    ) -> None:
        c, db = app_db
        _, task_id, token = await _setup(db)
        files = {"file": ("r.pdf", _PDF)}
        upload = (
            await c.post(
                f"/api/uploads/{task_id}", files=files, headers=_auth(token)
            )
        ).json()

        r = await c.delete(
            f"/api/uploads/{upload['id']}", headers=_auth(token)
        )
        assert r.status_code == 204
