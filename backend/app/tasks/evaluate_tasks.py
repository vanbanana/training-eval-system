"""Celery evaluate_task wrapper - Epic 16.8."""

from __future__ import annotations

from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


def run_evaluate_sync(upload_id: int) -> dict[str, Any]:
    import asyncio

    from app.core.database import SessionLocal
    from app.services.evaluation_service import EvaluationService

    async def _run() -> dict[str, Any]:
        async with SessionLocal() as db:
            svc = EvaluationService()
            ev = await svc.auto_score(db, upload_id=upload_id)
            await db.commit()
            return {
                "evaluation_id": ev.id,
                "total_score": ev.total_score,
                "status": ev.status,
            }

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.evaluate_tasks.evaluate_task", bind=True)
def evaluate_task(self: Any, upload_id: int) -> dict[str, Any]:
    log.info("evaluate_task.start", upload_id=upload_id, task_id=self.request.id)
    out = run_evaluate_sync(upload_id)
    log.info("evaluate_task.done", upload_id=upload_id, **out)
    return out
