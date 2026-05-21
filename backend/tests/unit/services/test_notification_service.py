"""Epic 21.1/21.2/21.3 验收：NotificationService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import AuthorizationError
from app.services.notification_events import (
    EVALUATION_COMPLETED,
    TASK_PUBLISHED,
)
from app.services.notification_service import NotificationService
from tests.factories.user_factory import UserFactory


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


class TestSend:
    async def test_send_inserts_for_each(
        self, session: AsyncSession
    ) -> None:
        u1 = await UserFactory.create_async(session)
        u2 = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        n = await svc.send(
            session,
            recipient_ids=[u1.id, u2.id],
            event_type=TASK_PUBLISHED,
            title="新任务",
            content="请尽快完成",
        )
        await session.commit()
        assert n == 2
        items = await svc.list_for_user(session, user_id=u1.id)
        assert len(items) == 1
        assert items[0].title == "新任务"

    async def test_disabled_pref_skipped(
        self, session: AsyncSession
    ) -> None:
        u1 = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        await svc.set_preference(
            session,
            user_id=u1.id,
            event_type=TASK_PUBLISHED,
            enabled=False,
        )
        await session.commit()
        n = await svc.send(
            session,
            recipient_ids=[u1.id],
            event_type=TASK_PUBLISHED,
            title="x",
        )
        assert n == 0


class TestMarkRead:
    async def test_owner_can_mark(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        await svc.send(
            session,
            recipient_ids=[u.id],
            event_type=EVALUATION_COMPLETED,
            title="ok",
        )
        await session.commit()
        items = await svc.list_for_user(session, user_id=u.id)
        n = await svc.mark_read(session, notification_id=items[0].id, user_id=u.id)
        assert n.is_read

    async def test_other_forbidden(self, session: AsyncSession) -> None:
        u1 = await UserFactory.create_async(session)
        u2 = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        await svc.send(
            session,
            recipient_ids=[u1.id],
            event_type=EVALUATION_COMPLETED,
            title="ok",
        )
        await session.commit()
        items = await svc.list_for_user(session, user_id=u1.id)
        with pytest.raises(AuthorizationError):
            await svc.mark_read(
                session, notification_id=items[0].id, user_id=u2.id
            )


class TestMarkAllRead:
    async def test_marks_all(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        for _ in range(3):
            await svc.send(
                session,
                recipient_ids=[u.id],
                event_type=TASK_PUBLISHED,
                title="x",
            )
        await session.commit()
        n = await svc.mark_all_read(session, user_id=u.id)
        await session.commit()
        assert n == 3
        unread = await svc.get_unread_count(session, user_id=u.id)
        assert unread == 0


class TestPreferences:
    async def test_default_all_enabled(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        prefs = await svc.get_preferences(session, user_id=u.id)
        assert all(prefs.values())

    async def test_set_persists(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session)
        await session.commit()
        svc = NotificationService()
        await svc.set_preference(
            session, user_id=u.id, event_type=TASK_PUBLISHED, enabled=False
        )
        await session.commit()
        prefs = await svc.get_preferences(session, user_id=u.id)
        assert prefs[TASK_PUBLISHED] is False
