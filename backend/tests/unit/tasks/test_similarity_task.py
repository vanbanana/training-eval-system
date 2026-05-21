"""Epic 17.4 验收：similarity_task Celery 包装."""

from __future__ import annotations

import inspect

import pytest

from app.tasks import similarity_tasks


pytestmark = pytest.mark.unit


class TestSimilarityTaskWrapper:
    def test_registered(self) -> None:
        names = list(similarity_tasks.celery_app.tasks.keys())
        assert "app.tasks.similarity_tasks.similarity_task" in names

    def test_run_signature(self) -> None:
        sig = inspect.signature(similarity_tasks.run_similarity_sync)
        assert "upload_id" in sig.parameters
