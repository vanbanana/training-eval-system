"""Epic 23.4 验收：audit_anomaly Celery 任务注册."""

from __future__ import annotations

import inspect

import pytest

from app.tasks import audit_anomaly


pytestmark = pytest.mark.unit


class TestAuditAnomaly:
    def test_registered(self) -> None:
        names = list(audit_anomaly.celery_app.tasks.keys())
        assert "app.tasks.audit_anomaly.audit_anomaly" in names

    def test_signature(self) -> None:
        sig = inspect.signature(audit_anomaly.run_audit_anomaly_sync)
        # 不需要参数
        assert len(sig.parameters) == 0


class TestAuditArchive:
    """Epic 23.6: CLI 入口函数存在."""

    def test_archive_func_exists(self) -> None:
        from app.cli.commands.audit_archive import archive_before, main

        assert callable(archive_before)
        assert callable(main)
