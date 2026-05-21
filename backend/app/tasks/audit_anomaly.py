"""可疑行为检测 Beat - Epic 23.4."""

from __future__ import annotations

from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


def run_audit_anomaly_sync() -> dict[str, Any]:
    import asyncio

    from app.core.database import SessionLocal
    from app.services.audit_service import AuditService
    from app.services.notification_events import (
        DEADLINE_APPROACHING,  # 占位事件，admin 仪表盘订阅
    )
    from app.services.notification_service import NotificationService

    async def _run() -> dict[str, Any]:
        async with SessionLocal() as db:
            audit_svc = AuditService()
            ids = await audit_svc.detect_suspicious_users(db)
            if not ids:
                return {"suspicious_users": []}
            from sqlalchemy import select

            from app.models.user import User

            admins = list(
                (
                    await db.execute(
                        select(User).where(User.role == "admin")
                    )
                )
                .scalars()
            )
            notify = NotificationService()
            await notify.send(
                db,
                recipient_ids=[a.id for a in admins],
                event_type=DEADLINE_APPROACHING,  # 简化：复用通用事件
                title="可疑用户告警",
                content=f"以下用户在 5 分钟内多次失败：{ids}",
                payload={"suspicious_users": ids},
            )
            await db.commit()
            return {"suspicious_users": ids}

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.audit_anomaly.audit_anomaly", bind=True)
def audit_anomaly(self: Any) -> dict[str, Any]:
    log.info("audit_anomaly.start")
    out = run_audit_anomaly_sync()
    log.info("audit_anomaly.done", **out)
    return out
