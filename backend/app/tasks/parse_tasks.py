"""Celery 解析任务 - 异步执行解析 + 自动触发核查.

流程：
1. 接收 upload_id
2. 执行 ParsePipeline.run()
3. 解析成功后自动触发 verify_task
4. 超时 120 秒（需求 5.1）
"""

from __future__ import annotations

import asyncio
from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


@celery_app.task(
    name="tasks.parse_upload",
    bind=True,
    max_retries=2,
    default_retry_delay=10,
    soft_time_limit=120,  # 需求 5.1: 120 秒超时
    time_limit=150,
)
def parse_upload_task(self: Any, upload_id: int) -> dict[str, Any]:
    """异步解析上传文件.

    成功后自动 enqueue verify_upload_task。
    """
    log.info("task.parse_upload.start", upload_id=upload_id, attempt=self.request.retries)

    try:
        result = asyncio.get_event_loop().run_until_complete(
            _run_parse(upload_id)
        )
        # 解析成功 → 自动触发核查（需求 6.1）
        from app.tasks.verify_tasks import verify_upload_task

        verify_upload_task.delay(upload_id)
        log.info("task.parse_upload.success", upload_id=upload_id)
        return {"upload_id": upload_id, "status": "parsed", "parse_result_id": result}

    except Exception as exc:
        log.error(
            "task.parse_upload.failed",
            upload_id=upload_id,
            error=str(exc),
            attempt=self.request.retries,
        )
        # 重试（指数退避由 Celery 管理）
        if self.request.retries < self.max_retries:
            raise self.retry(exc=exc)
        return {"upload_id": upload_id, "status": "failed", "error": str(exc)}


async def _run_parse(upload_id: int) -> int:
    """在 async 上下文中执行解析管线."""
    from app.core.config import get_settings
    from app.core.database import SessionLocal
    from app.llm.factory import llm_factory
    from app.services.parse_pipeline import ParsePipeline
    from app.storage import LocalFileStorage

    settings = get_settings()
    storage = LocalFileStorage(settings.upload_root)

    async with SessionLocal() as db:
        # 尝试获取 LLM provider（可能未配置）
        llm = None
        try:
            llm = await llm_factory.current(db)
        except Exception:  # noqa: BLE001
            log.info("task.parse_upload.no_llm", upload_id=upload_id)

        # 尝试获取 Redis（用于进度推送）
        redis = None
        try:
            from app.core.redis import get_redis

            redis = await get_redis()
        except Exception:  # noqa: BLE001
            pass

        pipeline = ParsePipeline(
            storage=storage,
            llm=llm,
            redis=redis,
        )
        result = await pipeline.run(db, upload_id=upload_id)
        await db.commit()
        return result.id
