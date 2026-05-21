"""Epic 13 验收：Celery 应用配置."""

from __future__ import annotations

import pytest

from app.core.config import get_settings
from app.tasks.celery_app import celery_app, create_celery_app


pytestmark = pytest.mark.unit


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


class TestCeleryConfig:
    def test_app_singleton(self) -> None:
        assert celery_app is not None
        assert celery_app.main == "tes"

    def test_create_app_returns_celery(self) -> None:
        app = create_celery_app()
        assert app.main == "tes"

    def test_serializer_is_json(self) -> None:
        assert celery_app.conf.task_serializer == "json"
        assert celery_app.conf.result_serializer == "json"
        assert "json" in celery_app.conf.accept_content

    def test_timezone_utc(self) -> None:
        assert celery_app.conf.timezone == "UTC"
        assert celery_app.conf.enable_utc is True

    def test_time_limits_set(self) -> None:
        assert celery_app.conf.task_time_limit == 600
        assert celery_app.conf.task_soft_time_limit == 540

    def test_routes_configured(self) -> None:
        routes = celery_app.conf.task_routes
        assert routes is not None
        assert "app.tasks.parse_tasks.*" in routes
        assert routes["app.tasks.parse_tasks.*"]["queue"] == "parse"
        assert "app.tasks.score_tasks.*" in routes
        assert "app.tasks.cleanup_tasks.*" in routes


class TestTaskRegistration:
    def test_task_decorator_works(self) -> None:
        from app.tasks.celery_app import celery_app as app

        @app.task(name="test.echo")
        def echo(x: str) -> str:
            return x

        assert "test.echo" in app.tasks
