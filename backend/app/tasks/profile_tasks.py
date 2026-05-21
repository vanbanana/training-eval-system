"""Celery profile_task wrapper - Epic 18.4."""

from __future__ import annotations

from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


def run_profile_sync(student_id: int) -> dict[str, Any]:
    import asyncio

    from app.core.database import SessionLocal
    from app.services.profile_service import (
        InsufficientDataError,
        ProfileService,
    )

    async def _run() -> dict[str, Any]:
        async with SessionLocal() as db:
            svc = ProfileService()
            try:
                p = await svc.compute_student_profile(db, student_id=student_id)
                await db.commit()
                return {
                    "student_id": student_id,
                    "profile_id": p.id,
                    "evaluation_count": p.source_evaluation_count,
                }
            except InsufficientDataError as e:
                return {"student_id": student_id, "skipped": True, "reason": str(e)}

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.profile_tasks.profile_task", bind=True)
def profile_task(self: Any, student_id: int) -> dict[str, Any]:
    log.info("profile_task.start", student_id=student_id)
    out = run_profile_sync(student_id)
    log.info("profile_task.done", **out)
    return out
