"""Celery similarity_task wrapper - Epic 17.4."""

from __future__ import annotations

from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


def run_similarity_sync(upload_id: int) -> dict[str, Any]:
    import asyncio

    from app.core.database import SessionLocal
    from app.services.similarity_service import SimilarityEngine

    async def _run() -> dict[str, Any]:
        async with SessionLocal() as db:
            engine = SimilarityEngine()
            recs = await engine.detect_for_upload(db, upload_id=upload_id)
            await db.commit()
            return {"upload_id": upload_id, "found": len(recs)}

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.similarity_tasks.similarity_task", bind=True)
def similarity_task(self: Any, upload_id: int) -> dict[str, Any]:
    log.info("similarity_task.start", upload_id=upload_id)
    out = run_similarity_sync(upload_id)
    log.info("similarity_task.done", **out)
    return out
