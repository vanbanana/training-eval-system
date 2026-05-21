"""NotificationService - Epic 21.2."""

from __future__ import annotations

from typing import Any

from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import AuthorizationError, ResourceNotFoundError
from app.core.logging import get_logger
from app.models.notification import Notification, NotificationPref


log = get_logger(__name__)


class NotificationService:
    async def send(
        self,
        db: AsyncSession,
        *,
        recipient_ids: list[int],
        event_type: str,
        title: str,
        content: str = "",
        payload: dict[str, Any] | None = None,
        link: str = "",
    ) -> int:
        """批量发送，返回成功 INSERT 的行数（跳过偏好关闭的）."""
        if not recipient_ids:
            return 0
        # 加载偏好
        prefs = list(
            (
                await db.execute(
                    select(NotificationPref).where(
                        NotificationPref.user_id.in_(recipient_ids),
                        NotificationPref.event_type == event_type,
                    )
                )
            )
            .scalars()
            .all()
        )
        disabled_ids = {p.user_id for p in prefs if not p.enabled}
        target_ids = [uid for uid in recipient_ids if uid not in disabled_ids]
        if not target_ids:
            return 0
        for uid in target_ids:
            db.add(
                Notification(
                    user_id=uid,
                    type=event_type,
                    title=title,
                    content=content,
                    payload=payload,
                    link=link,
                )
            )
        await db.flush()
        log.info(
            "notification.sent",
            event_type_=event_type,
            count=len(target_ids),
            disabled_count=len(disabled_ids),
        )
        return len(target_ids)

    async def list_for_user(
        self,
        db: AsyncSession,
        *,
        user_id: int,
        unread_only: bool = False,
        limit: int = 30,
    ) -> list[Notification]:
        stmt = select(Notification).where(Notification.user_id == user_id)
        if unread_only:
            stmt = stmt.where(Notification.is_read.is_(False))
        stmt = stmt.order_by(Notification.created_at.desc()).limit(limit)
        return list((await db.execute(stmt)).scalars().all())

    async def mark_read(
        self,
        db: AsyncSession,
        *,
        notification_id: int,
        user_id: int,
    ) -> Notification:
        n = await db.get(Notification, notification_id)
        if n is None:
            raise ResourceNotFoundError(
                f"notification {notification_id} not found"
            )
        if n.user_id != user_id:
            raise AuthorizationError("无权操作他人通知")
        n.is_read = True
        await db.flush()
        return n

    async def mark_all_read(
        self, db: AsyncSession, *, user_id: int
    ) -> int:
        result = await db.execute(
            update(Notification)
            .where(
                Notification.user_id == user_id,
                Notification.is_read.is_(False),
            )
            .values(is_read=True)
        )
        await db.flush()
        return int(result.rowcount or 0)

    async def get_unread_count(
        self, db: AsyncSession, *, user_id: int
    ) -> int:
        rows = list(
            (
                await db.execute(
                    select(Notification).where(
                        Notification.user_id == user_id,
                        Notification.is_read.is_(False),
                    )
                )
            )
            .scalars()
            .all()
        )
        return len(rows)

    async def get_preferences(
        self, db: AsyncSession, *, user_id: int
    ) -> dict[str, bool]:
        rows = list(
            (
                await db.execute(
                    select(NotificationPref).where(
                        NotificationPref.user_id == user_id
                    )
                )
            )
            .scalars()
            .all()
        )
        from app.services.notification_events import ALL_EVENTS

        out = {ev: True for ev in ALL_EVENTS}
        for r in rows:
            out[r.event_type] = r.enabled
        return out

    async def set_preference(
        self,
        db: AsyncSession,
        *,
        user_id: int,
        event_type: str,
        enabled: bool,
    ) -> None:
        existing = (
            await db.execute(
                select(NotificationPref).where(
                    NotificationPref.user_id == user_id,
                    NotificationPref.event_type == event_type,
                )
            )
        ).scalar_one_or_none()
        if existing is None:
            db.add(
                NotificationPref(
                    user_id=user_id,
                    event_type=event_type,
                    enabled=enabled,
                )
            )
        else:
            existing.enabled = enabled
        await db.flush()
