"""通知路由 - Epic 21.6."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel

from app.api.deps import CurrentUser, DbSession
from app.services.notification_service import NotificationService

router = APIRouter(prefix="/api/notifications", tags=["notifications"])


class PreferenceUpdate(BaseModel):
    event_type: str
    enabled: bool


@router.get("")
async def list_notifications(
    db: DbSession,
    current: CurrentUser,
    unread_only: bool = False,
    limit: int = 30,
) -> dict[str, object]:
    svc = NotificationService()
    items = await svc.list_for_user(
        db, user_id=current.id, unread_only=unread_only, limit=limit
    )
    unread = await svc.get_unread_count(db, user_id=current.id)
    return {
        "unread_count": unread,
        "items": [
            {
                "id": n.id,
                "type": n.type,
                "title": n.title,
                "content": n.content,
                "is_read": n.is_read,
                "link": n.link,
                "payload": n.payload,
                "created_at": n.created_at.isoformat(),
            }
            for n in items
        ],
    }


@router.post("/{notif_id}/read")
async def mark_read(
    notif_id: int, db: DbSession, current: CurrentUser
) -> dict[str, str]:
    svc = NotificationService()
    await svc.mark_read(
        db, notification_id=notif_id, user_id=current.id
    )
    await db.commit()
    return {"status": "ok"}


@router.post("/read-all")
async def mark_all_read(
    db: DbSession, current: CurrentUser
) -> dict[str, object]:
    svc = NotificationService()
    affected = await svc.mark_all_read(db, user_id=current.id)
    await db.commit()
    return {"status": "ok", "affected": affected}


@router.get("/preferences")
async def get_preferences(
    db: DbSession, current: CurrentUser
) -> dict[str, bool]:
    svc = NotificationService()
    return await svc.get_preferences(db, user_id=current.id)


@router.put("/preferences")
async def set_preference(
    payload: PreferenceUpdate, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    svc = NotificationService()
    await svc.set_preference(
        db,
        user_id=current.id,
        event_type=payload.event_type,
        enabled=payload.enabled,
    )
    await db.commit()
    return {"status": "ok", "event_type": payload.event_type, "enabled": payload.enabled}
