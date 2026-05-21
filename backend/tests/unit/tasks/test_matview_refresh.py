"""Epic 19.1/19.2 验收：物化视图刷新任务."""

from __future__ import annotations

import inspect

import pytest

from app.tasks import matview_refresh


pytestmark = pytest.mark.unit


class TestMatviewRefresh:
    def test_constants(self) -> None:
        assert "mv_class_progress" in matview_refresh.MATVIEWS
        assert "mv_course_metrics" in matview_refresh.MATVIEWS
        assert "mv_school_overview" in matview_refresh.MATVIEWS

    def test_celery_registered(self) -> None:
        names = list(matview_refresh.celery_app.tasks.keys())
        assert "app.tasks.matview_refresh.refresh_matview" in names
        assert "app.tasks.matview_refresh.refresh_all" in names

    def test_signatures(self) -> None:
        sig = inspect.signature(matview_refresh.refresh_matview_sync)
        assert "name" in sig.parameters

    def test_skips_on_sqlite(self) -> None:
        """Given SQLite db_url When refresh Then skipped=True."""
        out = matview_refresh.refresh_matview_sync("mv_class_progress")
        assert out.get("skipped") is True


class TestTeachingSummarySkill:
    def test_metadata(self) -> None:
        from app.llm.skills.profile import TeachingSummarySkill

        s = TeachingSummarySkill()
        assert s.name == "profile.teaching_summary"
        assert s.version == "1.0.0"
