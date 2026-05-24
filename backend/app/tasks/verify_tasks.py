"""Celery verify_task - 解析完成后自动触发核查.

核查引擎使用 LLM 进行深度覆盖度检查和逻辑漏洞检测。
失败重试 3 次（需求 6.7），连续失败标记为核查失败并通知教师。
"""

from __future__ import annotations

import asyncio
from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)


@celery_app.task(
    name="tasks.verify_upload",
    bind=True,
    max_retries=3,
    default_retry_delay=5,
    soft_time_limit=90,
    time_limit=120,
)
def verify_upload_task(self: Any, upload_id: int) -> dict[str, Any]:
    """Celery 任务：upload 解析完毕后核查.

    需求 6.7: LLM 调用失败重试 3 次，连续失败标记核查失败并通知教师。
    """
    log.info(
        "verify_task.start",
        upload_id=upload_id,
        attempt=self.request.retries,
    )
    try:
        result = asyncio.get_event_loop().run_until_complete(
            _run_verify(upload_id)
        )
        log.info("verify_task.done", upload_id=upload_id, **result)
        return result
    except Exception as exc:
        log.error(
            "verify_task.failed",
            upload_id=upload_id,
            error=str(exc),
            attempt=self.request.retries,
        )
        if self.request.retries < self.max_retries:
            raise self.retry(exc=exc)
        # 最终失败 → 通知教师
        asyncio.get_event_loop().run_until_complete(
            _notify_verify_failed(upload_id, str(exc))
        )
        return {"upload_id": upload_id, "status": "verify_failed", "error": str(exc)}


async def _run_verify(upload_id: int) -> dict[str, Any]:
    """在 async 上下文中执行核查引擎."""
    from app.core.database import SessionLocal
    from app.llm.factory import llm_factory
    from app.services.verify_engine import VerifyEngine

    async with SessionLocal() as db:
        # 尝试获取 LLM provider
        llm = None
        try:
            llm = await llm_factory.current(db)
        except Exception:  # noqa: BLE001
            log.info("verify_task.no_llm", upload_id=upload_id)

        engine = VerifyEngine(llm=llm)
        result = await engine.run(db, upload_id=upload_id)
        await db.commit()
        return {
            "upload_id": upload_id,
            "verify_result_id": result.id,
            "match_rate": float(result.match_rate or 0),
            "missing_count": len(result.missing_items or []),
            "logic_issues_count": len(result.logic_issues or []),
            "overall_confidence": result.overall_confidence or 0,
        }


async def _notify_verify_failed(upload_id: int, error: str) -> None:
    """核查最终失败时通知教师."""
    try:
        from app.core.database import SessionLocal
        from app.models.task import TrainingTask
        from app.models.upload import Upload

        async with SessionLocal() as db:
            upload = await db.get(Upload, upload_id)
            if upload is None:
                return
            task = await db.get(TrainingTask, upload.task_id)
            if task is None:
                return

            # 创建通知（如果通知服务可用）
            try:
                from app.services.notification_service import NotificationService

                svc = NotificationService()
                await svc.create(
                    db,
                    user_id=task.teacher_id,
                    title="核查失败",
                    content=f"学生上传 #{upload_id}（{upload.filename}）核查失败: {error[:200]}",
                    link=f"/teacher/tasks/{upload.task_id}/grading",
                )
                await db.commit()
            except Exception:  # noqa: BLE001
                pass
    except Exception:  # noqa: BLE001
        log.warning("verify_task.notify_failed", upload_id=upload_id)
