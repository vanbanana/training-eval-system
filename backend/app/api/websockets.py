"""WebSocket 端点 - Epic 21.5 通知推送."""

from __future__ import annotations

import asyncio

from fastapi import APIRouter, Query, WebSocket, WebSocketDisconnect, status
from sqlalchemy import select

from app.core.database import SessionLocal
from app.core.logging import get_logger
from app.core.security import decode_token
from app.models.notification import Notification


log = get_logger(__name__)
router = APIRouter()


async def _resolve_user_id(token: str) -> int | None:
    try:
        payload = decode_token(token)
        sub = payload.get("sub")
        if sub is None:
            return None
        return int(sub)
    except Exception:  # noqa: BLE001
        return None


@router.websocket("/ws/notify")
async def ws_notify(
    websocket: WebSocket, token: str = Query(...)
) -> None:
    """WebSocket 通知推送：定期轮询 DB 推送未读通知（dev 简化版）.

    生产替换为 Redis Pub/Sub。
    """
    user_id = await _resolve_user_id(token)
    if user_id is None:
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION)
        return

    await websocket.accept()
    last_id = 0
    log.info("ws.notify.connected", user_id=user_id)
    try:
        while True:
            async with SessionLocal() as db:
                rows = list(
                    (
                        await db.execute(
                            select(Notification)
                            .where(
                                Notification.user_id == user_id,
                                Notification.id > last_id,
                            )
                            .order_by(Notification.id)
                            .limit(20)
                        )
                    )
                    .scalars()
                    .all()
                )
            for n in rows:
                await websocket.send_json(
                    {
                        "id": n.id,
                        "type": n.type,
                        "title": n.title,
                        "content": n.content,
                        "is_read": n.is_read,
                        "payload": n.payload,
                    }
                )
                last_id = max(last_id, n.id)
            await asyncio.sleep(2.0)
    except WebSocketDisconnect:
        log.info("ws.notify.disconnected", user_id=user_id)
