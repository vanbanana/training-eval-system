"""Deadline reminder Celery task - Epic 21.4."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


def run_deadline_reminder_sync() -> dict[str, Any]:
    import asyncio

    from sqlalchemy import select
    from sqlalchemy.orm import selectinload

    from app.core.database import SessionLocal
    from app.models.course import ClassMembership
    from app.models.task import TrainingTask
    from app.models.upload import Upload
    from app.services.notification_events import DEADLINE_APPROACHING
    from app.services.notification_service import NotificationService

    async def _run() -> dict[str, Any]:
        async with SessionLocal() as db:
            now = datetime.now(UTC)
            window_start = now + timedelta(hours=23)
            window_end = now + timedelta(hours=24)
            tasks = list(
                (
                    await db.execute(
                        select(TrainingTask)
                        .options(selectinload(TrainingTask.classes))
                        .where(
                            TrainingTask.status == "published",
                            TrainingTask.deadline >= window_start,
                            TrainingTask.deadline <= window_end,
                        )
                    )
                )
                .scalars()
                .all()
            )
            svc = NotificationService()
            sent_total = 0
            for t in tasks:
                # 收集班级中的所有学生 id
                class_ids = [c.id for c in t.classes]
                if not class_ids:
                    continue
                roster = list(
                    (
                        await db.execute(
                            select(ClassMembership).where(
                                ClassMembership.class_id.in_(class_ids)
                            )
                        )
                    )
                    .scalars()
                    .all()
                )
                roster_ids = {r.student_id for r in roster}

                # 已提交的学生
                submitted = list(
                    (
                        await db.execute(
                            select(Upload.student_id).where(
                                Upload.task_id == t.id
                            )
                        )
                    ).scalars()
                )
                pending_ids = list(roster_ids - set(submitted))
                if not pending_ids:
                    continue
                count = await svc.send(
                    db,
                    recipient_ids=pending_ids,
                    event_type=DEADLINE_APPROACHING,
                    title=f"任务即将截止：{t.name}",
                    content=f"任务 {t.name} 将于 {t.deadline} 截止，请尽快提交。",
                    payload={"task_id": t.id},
                )
                sent_total += count
            await db.commit()
            return {"task_count": len(tasks), "sent_total": sent_total}

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.deadline_reminder.deadline_reminder", bind=True)
def deadline_reminder(self: Any) -> dict[str, Any]:
    log.info("deadline_reminder.start")
    out = run_deadline_reminder_sync()
    log.info("deadline_reminder.done", **out)
    return out
