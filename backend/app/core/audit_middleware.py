"""@audit 装饰器 + AuditMiddleware - Epic 23.3."""

from __future__ import annotations

import functools
from collections.abc import Awaitable, Callable
from typing import Any

from app.core.logging import get_logger


log = get_logger(__name__)


def audit(
    action: str,
    *,
    target_extractor: Callable[..., dict[str, Any]] | None = None,
) -> Callable[..., Any]:
    """API 路由级审计装饰器.

    用法：
        @audit("user.create", target_extractor=lambda payload, **k: {...})
        async def create_user(...):
            ...
    """

    def decorator(
        fn: Callable[..., Awaitable[Any]],
    ) -> Callable[..., Awaitable[Any]]:
        @functools.wraps(fn)
        async def wrapper(*args: Any, **kwargs: Any) -> Any:
            try:
                result = await fn(*args, **kwargs)
                _log("success", action, target_extractor, args, kwargs)
                return result
            except Exception as e:  # noqa: BLE001
                _log("failed", action, target_extractor, args, kwargs, error=str(e))
                raise

        return wrapper

    return decorator


def _log(
    result: str,
    action: str,
    extractor: Callable[..., dict[str, Any]] | None,
    args: tuple[Any, ...],
    kwargs: dict[str, Any],
    error: str | None = None,
) -> None:
    target_info: dict[str, Any] = {}
    if extractor is not None:
        try:
            target_info = extractor(*args, **kwargs)
        except Exception:  # noqa: BLE001
            target_info = {}
    log.info(
        "audit.action",
        action=action,
        result=result,
        target=target_info,
        error=error,
    )


__all__ = ["audit"]
