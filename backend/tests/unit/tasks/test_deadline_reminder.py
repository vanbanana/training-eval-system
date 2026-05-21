"""Epic 21.4 验收：deadline_reminder Celery 任务."""

from __future__ import annotations

import inspect

import pytest

from app.tasks import deadline_reminder


pytestmark = pytest.mark.unit


class TestDeadlineReminder:
    def test_registered(self) -> None:
        names = list(deadline_reminder.celery_app.tasks.keys())
        assert "app.tasks.deadline_reminder.deadline_reminder" in names

    def test_signature(self) -> None:
        sig = inspect.signature(deadline_reminder.run_deadline_reminder_sync)
        assert sig.parameters == {} or "self" in sig.parameters or len(sig.parameters) == 0
