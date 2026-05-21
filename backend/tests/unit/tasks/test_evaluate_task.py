"""Epic 16.8 验收：evaluate_task Celery 包装."""

from __future__ import annotations

import inspect

import pytest

from app.tasks import evaluate_tasks


pytestmark = pytest.mark.unit


class TestEvaluateTaskWrapper:
    def test_registered(self) -> None:
        names = list(evaluate_tasks.celery_app.tasks.keys())
        assert "app.tasks.evaluate_tasks.evaluate_task" in names

    def test_run_signature(self) -> None:
        sig = inspect.signature(evaluate_tasks.run_evaluate_sync)
        assert "upload_id" in sig.parameters
