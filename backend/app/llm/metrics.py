"""LLMMetrics 装饰器 - 记录耗时 + tokens."""

from __future__ import annotations

import time
from collections import defaultdict
from collections.abc import Awaitable, Callable
from functools import wraps
from typing import Any, TypeVar

from app.core.logging import get_logger
from app.llm.base import LLMResponse


T = TypeVar("T")
log = get_logger(__name__)


# 简易内存计数器（生产用 Prometheus）
_counters: dict[str, int] = defaultdict(int)
_durations_ms: dict[str, list[float]] = defaultdict(list)


def reset_metrics() -> None:
    _counters.clear()
    _durations_ms.clear()


def get_metrics() -> dict[str, object]:
    return {
        "counters": dict(_counters),
        "durations_ms": {k: list(v) for k, v in _durations_ms.items()},
    }


def with_metrics(
    op: str,
) -> Callable[[Callable[..., Awaitable[T]]], Callable[..., Awaitable[T]]]:
    """记录调用耗时和 tokens."""

    def decorator(
        func: Callable[..., Awaitable[T]],
    ) -> Callable[..., Awaitable[T]]:
        @wraps(func)
        async def wrapper(*args: Any, **kwargs: Any) -> T:
            start = time.perf_counter()
            try:
                result = await func(*args, **kwargs)
                duration_ms = (time.perf_counter() - start) * 1000
                _counters[f"llm.call.{op}.success"] += 1
                _durations_ms[op].append(duration_ms)

                tokens = 0
                if isinstance(result, LLMResponse):
                    tokens = result.prompt_tokens + result.completion_tokens
                    _counters["llm.tokens.total"] += tokens
                log.info(
                    "llm.call.success",
                    op=op,
                    duration_ms=int(duration_ms),
                    tokens=tokens,
                )
                return result
            except Exception as e:
                duration_ms = (time.perf_counter() - start) * 1000
                _counters[f"llm.call.{op}.failed"] += 1
                _durations_ms[op].append(duration_ms)
                log.warning(
                    "llm.call.failed",
                    op=op,
                    duration_ms=int(duration_ms),
                    error=str(e),
                )
                raise

        return wrapper

    return decorator
