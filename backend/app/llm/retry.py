"""LLM 重试 + 熔断装饰器."""

from __future__ import annotations

import asyncio
import time
from collections.abc import Awaitable, Callable
from functools import wraps
from typing import Any, TypeVar

from app.core.exceptions import LLMUnavailableError
from app.core.logging import get_logger


T = TypeVar("T")
log = get_logger(__name__)


# ============ 简单熔断器 ============


class CircuitBreaker:
    """状态：closed -> open -> half_open -> closed."""

    def __init__(
        self,
        *,
        failure_threshold: int = 5,
        recovery_seconds: float = 30,
    ) -> None:
        self.failure_threshold = failure_threshold
        self.recovery_seconds = recovery_seconds
        self._failures = 0
        self._opened_at: float | None = None

    @property
    def state(self) -> str:
        if self._opened_at is None:
            return "closed"
        if time.monotonic() - self._opened_at >= self.recovery_seconds:
            return "half_open"
        return "open"

    def allow(self) -> bool:
        return self.state in ("closed", "half_open")

    def on_success(self) -> None:
        self._failures = 0
        self._opened_at = None

    def on_failure(self) -> None:
        self._failures += 1
        if self._failures >= self.failure_threshold:
            self._opened_at = time.monotonic()
            log.warning(
                "llm.circuit_breaker.opened",
                failures=self._failures,
                threshold=self.failure_threshold,
            )


# 全局单例
default_breaker = CircuitBreaker()


# ============ 重试装饰器 ============


def _is_retryable(exc: BaseException) -> bool:
    """4xx (除 429) 不重试；5xx / 网络 / 429 重试."""
    msg = str(exc).lower()
    if "429" in msg or "rate" in msg or "timeout" in msg or "5" in msg[:5]:
        return True
    if "400" in msg or "401" in msg or "403" in msg or "404" in msg:
        return False
    return True  # 默认重试（网络异常等）


def with_retry(
    *,
    max_attempts: int = 3,
    initial_delay: float = 1.0,
    max_delay: float = 10.0,
    breaker: CircuitBreaker | None = None,
) -> Callable[[Callable[..., Awaitable[T]]], Callable[..., Awaitable[T]]]:
    """指数退避重试 + 熔断保护."""

    def decorator(
        func: Callable[..., Awaitable[T]],
    ) -> Callable[..., Awaitable[T]]:
        @wraps(func)
        async def wrapper(*args: Any, **kwargs: Any) -> T:
            cb = breaker or default_breaker
            if not cb.allow():
                raise LLMUnavailableError(
                    "LLM 熔断中，请稍后重试", field="circuit_breaker"
                )

            last_exc: BaseException | None = None
            for attempt in range(1, max_attempts + 1):
                try:
                    result = await func(*args, **kwargs)
                    cb.on_success()
                    return result
                except Exception as e:  # noqa: BLE001
                    last_exc = e
                    if not _is_retryable(e) or attempt == max_attempts:
                        cb.on_failure()
                        raise
                    delay = min(initial_delay * (2 ** (attempt - 1)), max_delay)
                    log.info(
                        "llm.retry",
                        attempt=attempt,
                        delay=delay,
                        error=str(e),
                    )
                    await asyncio.sleep(delay)

            assert last_exc is not None
            raise last_exc

        return wrapper

    return decorator
