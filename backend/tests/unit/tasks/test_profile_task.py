"""Epic 18.4 验收：profile_task Celery 包装."""

from __future__ import annotations

import inspect

import pytest

from app.tasks import profile_tasks


pytestmark = pytest.mark.unit


class TestProfileTaskWrapper:
    def test_registered(self) -> None:
        names = list(profile_tasks.celery_app.tasks.keys())
        assert "app.tasks.profile_tasks.profile_task" in names

    def test_run_signature(self) -> None:
        sig = inspect.signature(profile_tasks.run_profile_sync)
        assert "student_id" in sig.parameters
