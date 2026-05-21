"""Celery verify_task wrapper - Epic 15.3."""

from __future__ import annotations

from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


def run_verify_sync(upload_id: int) -> dict[str, Any]:
    """同步入口（被 Celery worker 或单元测试调用）.

    实际异步实现委托 verify_engine。这里仅作为 Celery 入口，
    内部驱动一个临时 asyncio loop 调 VerifyEngine.run。
    """
    import asyncio

    from app.core.database import SessionLocal
    from app.services.verify_engine import VerifyEngine

    async def _run() -> dict[str, Any]:
        async with SessionLocal() as db:
            engine = VerifyEngine()
            result = await engine.run(db, upload_id=upload_id)
            await db.commit()
            return {
                "verify_result_id": result.id,
                "match_rate": float(result.match_rate or 0),
                "missing_count": len(result.missing_items or []),
            }

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.verify_tasks.verify_task", bind=True)
def verify_task(self: Any, upload_id: int) -> dict[str, Any]:
    """Celery 任务：upload 解析完毕后核查."""
    log.info("verify_task.start", upload_id=upload_id, task_id=self.request.id)
    out = run_verify_sync(upload_id)
    log.info("verify_task.done", upload_id=upload_id, **out)
    return out
