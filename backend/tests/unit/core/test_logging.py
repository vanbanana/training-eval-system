"""Task 1.3 验收：结构化日志."""

from __future__ import annotations

import io
import json
import logging
import re

import pytest
import structlog

from app.core.logging import (
    bind_request_context,
    clear_request_context,
    configure_logging,
    get_logger,
    trace_id_ctx,
    user_id_ctx,
)


@pytest.fixture()
def json_logger() -> tuple[structlog.stdlib.BoundLogger, io.StringIO]:
    """配置成 JSON 输出，捕获 stdout 到 buffer."""
    # 重置 structlog
    structlog.reset_defaults()
    buf = io.StringIO()
    handler = logging.StreamHandler(buf)
    root = logging.getLogger()
    # 清掉 stream handlers 防止重复
    for h in list(root.handlers):
        root.removeHandler(h)
    root.addHandler(handler)
    root.setLevel(logging.INFO)

    configure_logging(level="INFO", env="prod")  # prod 用 JSONRenderer
    log = get_logger("test")
    yield log, buf

    clear_request_context()


def _last_json_line(buf: io.StringIO) -> dict[str, object]:
    text = buf.getvalue().strip()
    assert text, "no log output"
    return json.loads(text.splitlines()[-1])


class TestLoggerHappyPath:
    def test_logger_outputs_json_with_trace_id(
        self, json_logger: tuple[structlog.stdlib.BoundLogger, io.StringIO]
    ) -> None:
        """Given 已 bind_request_context；When log.info；Then JSON 含 trace_id/user_id/event/extras."""
        log, buf = json_logger
        bind_request_context(trace_id="abc-123", user_id=42)

        log.info("upload.create.start", file_size=1024)

        parsed = _last_json_line(buf)
        assert parsed["trace_id"] == "abc-123"
        assert parsed["user_id"] == 42
        assert parsed["event"] == "upload.create.start"
        assert parsed["file_size"] == 1024
        assert parsed["level"] == "info"


class TestSensitiveRedaction:
    def test_password_and_api_key_redacted(
        self, json_logger: tuple[structlog.stdlib.BoundLogger, io.StringIO]
    ) -> None:
        """Given 日志含 password/api_key；When 输出；Then 替换为 ***，原值不出现。"""
        log, buf = json_logger
        bind_request_context(trace_id="t-1")

        log.info("user.login", password="secret123", api_key="sk-abc")

        parsed = _last_json_line(buf)
        assert parsed["password"] == "***"
        assert parsed["api_key"] == "***"
        # 原值不出现在整行 JSON 中
        full_line = buf.getvalue()
        assert "secret123" not in full_line
        assert "sk-abc" not in full_line


class TestNoTraceContext:
    def test_no_trace_context_returns_empty_not_lookup_error(
        self, json_logger: tuple[structlog.stdlib.BoundLogger, io.StringIO]
    ) -> None:
        """Given 未 bind 上下文；When log.info；Then trace_id 为空字符串而非 LookupError。"""
        log, buf = json_logger
        clear_request_context()

        log.info("event")

        parsed = _last_json_line(buf)
        assert "trace_id" in parsed
        assert parsed["trace_id"] == ""


class TestContextVarsIsolation:
    def test_context_does_not_leak_between_clear_calls(self) -> None:
        """Given bind 后 clear；When 读取；Then 还原为默认值。"""
        bind_request_context(trace_id="t1", user_id=99)
        assert trace_id_ctx.get() == "t1"
        assert user_id_ctx.get() == 99

        clear_request_context()
        assert trace_id_ctx.get() == ""
        assert user_id_ctx.get() is None


class TestLogLevels:
    def test_warning_level_emitted(
        self, json_logger: tuple[structlog.stdlib.BoundLogger, io.StringIO]
    ) -> None:
        log, buf = json_logger
        log.warning("auth.login.failed", reason="bad password")
        parsed = _last_json_line(buf)
        assert parsed["level"] == "warning"
        assert parsed["event"] == "auth.login.failed"

    def test_iso_timestamp_format(
        self, json_logger: tuple[structlog.stdlib.BoundLogger, io.StringIO]
    ) -> None:
        log, buf = json_logger
        log.info("event")
        parsed = _last_json_line(buf)
        ts = parsed["timestamp"]
        assert isinstance(ts, str)
        # ISO 8601 格式，含 T 与 Z 或 +
        assert re.match(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}", ts)
