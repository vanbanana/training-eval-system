"""结构化日志 - JSON 输出 + trace_id 透传 + 敏感字段过滤."""

from __future__ import annotations

import logging
import sys
from contextvars import ContextVar
from typing import Any

import structlog

trace_id_ctx: ContextVar[str] = ContextVar("trace_id", default="")
user_id_ctx: ContextVar[int | None] = ContextVar("user_id", default=None)

_SENSITIVE_KEYS = {
    "password",
    "password_hash",
    "api_key",
    "secret",
    "secret_key",
    "token",
    "authorization",
    "jwt_secret",
    "llm_key_master",
}


def _add_trace_context(
    _logger: object, _method: str, event_dict: dict[str, Any]
) -> dict[str, Any]:
    event_dict.setdefault("trace_id", trace_id_ctx.get())
    user_id = user_id_ctx.get()
    if user_id is not None:
        event_dict.setdefault("user_id", user_id)
    return event_dict


def _redact_sensitive(
    _logger: object, _method: str, event_dict: dict[str, Any]
) -> dict[str, Any]:
    for key in list(event_dict.keys()):
        if key.lower() in _SENSITIVE_KEYS:
            event_dict[key] = "***"
    return event_dict


def configure_logging(level: str = "INFO", env: str = "dev") -> None:
    """初始化 structlog（应用启动时调用一次）."""
    logging.basicConfig(
        format="%(message)s",
        stream=sys.stdout,
        level=getattr(logging, level.upper(), logging.INFO),
    )
    processors: list[Any] = [
        structlog.contextvars.merge_contextvars,
        structlog.stdlib.add_log_level,
        structlog.processors.TimeStamper(fmt="iso", utc=True),
        _add_trace_context,
        _redact_sensitive,
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
    ]
    if env == "dev":
        processors.append(structlog.dev.ConsoleRenderer(colors=False))
    else:
        processors.append(structlog.processors.JSONRenderer())
    structlog.configure(
        processors=processors,
        logger_factory=structlog.stdlib.LoggerFactory(),
        wrapper_class=structlog.stdlib.BoundLogger,
        cache_logger_on_first_use=True,
    )


def get_logger(name: str) -> structlog.stdlib.BoundLogger:
    return structlog.get_logger(name)


def bind_request_context(*, trace_id: str, user_id: int | None = None) -> None:
    trace_id_ctx.set(trace_id)
    user_id_ctx.set(user_id)


def clear_request_context() -> None:
    trace_id_ctx.set("")
    user_id_ctx.set(None)
