"""Epic 23.2 / 23.4 验收：AuditService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.services.audit_service import AuditService


pytestmark = pytest.mark.unit


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestAuditService:
    async def test_emit_persists(self, session: AsyncSession) -> None:
        svc = AuditService()
        row = await svc.emit(
            session,
            action="user.create",
            target_type="user",
            target_id=42,
            user_id=1,
            username="admin",
            role="admin",
            client_ip="127.0.0.1",
        )
        await session.commit()
        assert row.id is not None
        assert row.target == "user:42"

    async def test_list_filter_by_action(
        self, session: AsyncSession
    ) -> None:
        svc = AuditService()
        for action in ("user.create", "task.publish", "user.create"):
            await svc.emit(session, action=action, user_id=1)
        await session.commit()
        rows = await svc.list_logs(session, action="user.create")
        assert len(rows) == 2

    async def test_detect_suspicious_users(
        self, session: AsyncSession
    ) -> None:
        svc = AuditService()
        # 9 次失败 → 不触发
        for _ in range(9):
            await svc.emit(
                session, action="auth.login", result="failed", user_id=7
            )
        await session.commit()
        flagged = await svc.detect_suspicious_users(session)
        assert flagged == []

        # 第 10 次失败 → 触发
        await svc.emit(
            session, action="auth.login", result="failed", user_id=7
        )
        await session.commit()
        flagged = await svc.detect_suspicious_users(session)
        assert 7 in flagged


class TestAuditDecorator:
    async def test_decorator_logs_success(self) -> None:
        from app.core.audit_middleware import audit

        calls = []

        @audit("test.action")
        async def fn(x: int) -> int:
            calls.append(x)
            return x * 2

        result = await fn(3)
        assert result == 6
        assert calls == [3]
