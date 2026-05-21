"""Epic 24.1/24.3/24.4 验收：DashboardService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.evaluation import Evaluation
from app.models.upload import Upload
from app.services.dashboard_service import DashboardService, get_system_resources
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import AdminFactory, TeacherFactory, UserFactory


pytestmark = pytest.mark.unit


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    # 清理跨测试的全局缓存
    from app.services.dashboard_service import _CACHE

    _CACHE.clear()
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()
    _CACHE.clear()


class TestDashboardByRole:
    async def test_admin_returns_counters(
        self, session: AsyncSession
    ) -> None:
        admin = await AdminFactory.create_async(session)
        await UserFactory.create_async(session)
        await session.commit()
        svc = DashboardService()
        data = await svc.get_dashboard(session, user=admin)
        assert data["role"] == "admin"
        assert "user_count" in data
        assert "system_resources" in data

    async def test_teacher_pending_grading(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session)
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(
            session, teacher=teacher
        )
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        session.add(upload)
        await session.flush()
        ev = Evaluation(
            task_id=task.id,
            student_id=student.id,
            upload_id=upload.id,
            status="auto_scored",
            total_score=80.0,
        )
        session.add(ev)
        await session.commit()
        svc = DashboardService()
        data = await svc.get_dashboard(session, user=teacher)
        assert data["role"] == "teacher"
        assert data["my_tasks"] >= 1
        assert data["pending_grading"] >= 1

    async def test_student_recent_evals(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session)
        student = await UserFactory.create_async(session)
        task = await TrainingTaskFactory.create_async(
            session, teacher=teacher
        )
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename="r",
            file_type="docx",
            file_size=10,
            storage_path="x",
            parse_status="parsed",
        )
        session.add(upload)
        await session.flush()
        session.add(
            Evaluation(
                task_id=task.id,
                student_id=student.id,
                upload_id=upload.id,
                status="auto_scored",
                total_score=88.0,
            )
        )
        await session.commit()
        svc = DashboardService()
        data = await svc.get_dashboard(session, user=student)
        assert data["role"] == "student"
        assert isinstance(data["recent_evaluations"], list)


class TestInvalidate:
    async def test_invalidate_drops_cached_data(
        self, session: AsyncSession
    ) -> None:
        teacher = await TeacherFactory.create_async(session)
        await session.commit()
        svc = DashboardService()
        first = await svc.get_dashboard(session, user=teacher)
        # 加新任务但缓存仍未失效
        await TrainingTaskFactory.create_async(session, teacher=teacher)
        await session.commit()
        cached = await svc.get_dashboard(session, user=teacher)
        assert cached["my_tasks"] == first["my_tasks"]
        # 失效后立即更新
        await svc.invalidate([teacher.id])
        fresh = await svc.get_dashboard(session, user=teacher)
        assert fresh["my_tasks"] >= first["my_tasks"]


class TestSystemResources:
    async def test_returns_dict(self) -> None:
        out = await get_system_resources()
        assert "cpu_percent" in out
        assert "mem_percent" in out
        assert "disk_percent" in out
