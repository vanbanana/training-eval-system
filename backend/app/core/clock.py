"""Clock 抽象 - Epic 26.6.

业务代码注入 Clock 而非直接调用 datetime.now，便于测试和 dev 端点冻结时间。
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Protocol


class Clock(Protocol):
    def now(self) -> datetime: ...

    def utcnow(self) -> datetime: ...


class SystemClock:
    def now(self) -> datetime:
        return datetime.now(UTC)

    def utcnow(self) -> datetime:
        return datetime.now(UTC)


class FrozenClock:
    """测试用：可手动 set/advance."""

    def __init__(self, fixed: datetime | None = None) -> None:
        self._t = fixed or datetime(2026, 1, 1, tzinfo=UTC)

    def now(self) -> datetime:
        return self._t

    def utcnow(self) -> datetime:
        return self._t

    def set(self, t: datetime) -> None:
        if t.tzinfo is None:
            t = t.replace(tzinfo=UTC)
        self._t = t

    def advance(self, *, seconds: int = 0, minutes: int = 0, hours: int = 0) -> None:
        self._t = self._t + timedelta(seconds=seconds, minutes=minutes, hours=hours)


_clock: Clock = SystemClock()


def get_clock() -> Clock:
    return _clock


def set_clock(c: Clock) -> None:
    global _clock
    _clock = c


__all__ = [
    "Clock",
    "FrozenClock",
    "SystemClock",
    "get_clock",
    "set_clock",
]
