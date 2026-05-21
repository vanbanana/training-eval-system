"""Celery 应用实例."""

from __future__ import annotations

from celery import Celery

from app.core.config import get_settings


def create_celery_app() -> Celery:
    settings = get_settings()
    app = Celery(
        "tes",
        broker=settings.redis_url,
        backend=settings.redis_url,
    )
    app.conf.update(
        task_serializer="json",
        accept_content=["json"],
        result_serializer="json",
        timezone="UTC",
        enable_utc=True,
        task_track_started=True,
        task_time_limit=600,
        task_soft_time_limit=540,
        worker_prefetch_multiplier=1,
        broker_connection_retry_on_startup=True,
    )
    # 默认队列分组：parse / score / report / cleanup
    app.conf.task_routes = {
        "app.tasks.parse_tasks.*": {"queue": "parse"},
        "app.tasks.score_tasks.*": {"queue": "score"},
        "app.tasks.report_tasks.*": {"queue": "report"},
        "app.tasks.cleanup_tasks.*": {"queue": "cleanup"},
    }
    return app


celery_app = create_celery_app()
