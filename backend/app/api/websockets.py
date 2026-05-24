"""WebSocket 端点 - 通知推送 + 解析进度推送."""

from __future__ import annotations

import asyncio
import json

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


@router.websocket("/ws/progress")
async def ws_progress(
    websocket: WebSocket, token: str = Query(...)
) -> None:
    """WebSocket 解析/评价进度推送.

    推送格式：
    {
        "upload_id": 123,
        "status": "parsing" | "parsed" | "failed" | "scoring" | "scored",
        "progress": 0-100,
        "error": null | "错误信息"
    }

    实现策略：
    - 优先使用 Redis Pub/Sub（如果 Redis 可用）
    - 降级为 DB 轮询（dev 环境）
    """
    user_id = await _resolve_user_id(token)
    if user_id is None:
        await websocket.close(code=status.WS_1008_POLICY_VIOLATION)
        return

    await websocket.accept()
    log.info("ws.progress.connected", user_id=user_id)

    # 尝试 Redis Pub/Sub 模式
    redis_mode = False
    try:
        from app.core.redis import get_redis
        from app.services.progress_pubsub import subscribe_for_user

        redis = await get_redis()
        pubsub = await subscribe_for_user(redis, user_id)
        redis_mode = True
        log.info("ws.progress.redis_mode", user_id=user_id)
    except Exception:  # noqa: BLE001
        log.info("ws.progress.polling_mode", user_id=user_id)

    try:
        if redis_mode:
            # Redis Pub/Sub 模式：实时推送
            async for msg in pubsub.listen():  # type: ignore[union-attr]
                if msg["type"] == "message":
                    data = msg["data"]
                    if isinstance(data, bytes):
                        data = data.decode("utf-8")
                    await websocket.send_text(data)
        else:
            # DB 轮询降级模式：每 2 秒检查上传状态变化
            from app.models.upload import Upload

            known_states: dict[int, str] = {}
            while True:
                async with SessionLocal() as db:
                    uploads = list(
                        (
                            await db.execute(
                                select(Upload)
                                .where(
                                    Upload.student_id == user_id,
                                    Upload.parse_status.in_(["pending", "parsing"]),
                                )
                                .order_by(Upload.created_at.desc())
                                .limit(10)
                            )
                        )
                        .scalars()
                        .all()
                    )
                    # 也检查刚完成的（状态刚变为 parsed/failed）
                    recently_done = list(
                        (
                            await db.execute(
                                select(Upload)
                                .where(
                                    Upload.student_id == user_id,
                                    Upload.parse_status.in_(["parsed", "failed"]),
                                )
                                .order_by(Upload.updated_at.desc())
                                .limit(5)
                            )
                        )
                        .scalars()
                        .all()
                    )

                all_uploads = uploads + recently_done
                for u in all_uploads:
                    old_state = known_states.get(u.id)
                    if old_state != u.parse_status:
                        known_states[u.id] = u.parse_status
                        progress = {
                            "pending": 0,
                            "parsing": 50,
                            "parsed": 100,
                            "failed": 0,
                        }.get(u.parse_status, 0)
                        await websocket.send_json({
                            "upload_id": u.id,
                            "status": u.parse_status,
                            "progress": progress,
                            "error": None,
                        })

                await asyncio.sleep(2.0)
    except WebSocketDisconnect:
        log.info("ws.progress.disconnected", user_id=user_id)
    finally:
        if redis_mode:
            try:
                await pubsub.unsubscribe()  # type: ignore[union-attr]
            except Exception:  # noqa: BLE001
                pass
