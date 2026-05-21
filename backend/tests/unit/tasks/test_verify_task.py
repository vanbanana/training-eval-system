"""Epic 15.3 验收：verify_task Celery 包装."""

from __future__ import annotations

import pytest

from app.tasks import verify_tasks


pytestmark = pytest.mark.unit


class TestVerifyTaskWrapper:
    def test_wrapper_registered(self) -> None:
        """Given Celery app When list registered tasks Then verify_task 在."""
        names = list(verify_tasks.celery_app.tasks.keys())
        assert "app.tasks.verify_tasks.verify_task" in names

    def test_run_verify_sync_signature(self) -> None:
        """Given run_verify_sync When 检查签名 Then 接受 upload_id."""
        import inspect

        sig = inspect.signature(verify_tasks.run_verify_sync)
        assert "upload_id" in sig.parameters
