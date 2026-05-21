"""Matview refresh tasks - Epic 19.2.

PG 物化视图（mv_class_progress / mv_course_metrics / mv_school_overview）由
Alembic 迁移创建（见 alembic/versions/xxxx_create_matviews.py，PG 专用 SQL）。
SQLite 上跳过执行（任务返回 skipped 状态）。
"""

from __future__ import annotations

import time
from typing import Any

from app.core.logging import get_logger
from app.tasks.celery_app import celery_app


log = get_logger(__name__)

MATVIEWS = (
    "mv_class_progress",
    "mv_course_metrics",
    "mv_school_overview",
)


def refresh_matview_sync(name: str) -> dict[str, Any]:
    """同步执行单个 matview 刷新；非 PG 直接跳过."""
    import asyncio

    from app.core.database import SessionLocal
    from app.core.config import get_settings

    async def _run() -> dict[str, Any]:
        settings = get_settings()
        if "postgres" not in settings.db_url:
            return {"matview": name, "skipped": True, "reason": "non-postgres"}
        from sqlalchemy import text as sa_text

        start = time.perf_counter()
        async with SessionLocal() as db:
            try:
                await db.execute(
                    sa_text(f"REFRESH MATERIALIZED VIEW CONCURRENTLY {name}")
                )
                await db.commit()
            except Exception as e:  # noqa: BLE001
                # 退化到 non-concurrent
                try:
                    await db.execute(
                        sa_text(f"REFRESH MATERIALIZED VIEW {name}")
                    )
                    await db.commit()
                except Exception as e2:  # noqa: BLE001
                    return {
                        "matview": name,
                        "ok": False,
                        "error": str(e2),
                    }
        return {
            "matview": name,
            "ok": True,
            "duration_ms": int((time.perf_counter() - start) * 1000),
        }

    return asyncio.run(_run())


@celery_app.task(name="app.tasks.matview_refresh.refresh_matview", bind=True)
def refresh_matview(self: Any, name: str) -> dict[str, Any]:
    log.info("matview.refresh.start", matview=name)
    out = refresh_matview_sync(name)
    log.info("matview.refresh.done", **out)
    return out


@celery_app.task(name="app.tasks.matview_refresh.refresh_all", bind=True)
def refresh_all(self: Any) -> list[dict[str, Any]]:
    return [refresh_matview_sync(n) for n in MATVIEWS]
