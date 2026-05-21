"""Epic 26.6 验收：Clock + FrozenClock."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest

from app.core.clock import FrozenClock, SystemClock, get_clock, set_clock


pytestmark = pytest.mark.unit


class TestSystemClock:
    def test_now_returns_aware_datetime(self) -> None:
        c = SystemClock()
        t = c.now()
        assert t.tzinfo is not None


class TestFrozenClock:
    def test_default_fixed(self) -> None:
        c = FrozenClock()
        a = c.now()
        b = c.now()
        assert a == b

    def test_advance_60s(self) -> None:
        c = FrozenClock(datetime(2026, 1, 1, tzinfo=UTC))
        c.advance(seconds=60)
        assert c.now() == datetime(2026, 1, 1, 0, 1, tzinfo=UTC)

    def test_set(self) -> None:
        c = FrozenClock()
        new_t = datetime(2030, 6, 1, 12, tzinfo=UTC)
        c.set(new_t)
        assert c.now() == new_t


class TestGlobalClock:
    def test_set_and_restore(self) -> None:
        original = get_clock()
        try:
            fc = FrozenClock()
            set_clock(fc)
            assert get_clock() is fc
        finally:
            set_clock(original)
