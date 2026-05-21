"""Task 10.3 验收：重试 + 熔断."""

from __future__ import annotations

import time

import pytest

from app.core.exceptions import LLMUnavailableError
from app.llm.retry import CircuitBreaker, with_retry


pytestmark = pytest.mark.unit


class TestCircuitBreaker:
    def test_default_state_closed(self) -> None:
        cb = CircuitBreaker()
        assert cb.state == "closed"
        assert cb.allow() is True

    def test_opens_after_threshold_failures(self) -> None:
        cb = CircuitBreaker(failure_threshold=3)
        for _ in range(3):
            cb.on_failure()
        assert cb.state == "open"
        assert cb.allow() is False

    def test_half_open_after_recovery(self) -> None:
        cb = CircuitBreaker(failure_threshold=2, recovery_seconds=0.05)
        cb.on_failure()
        cb.on_failure()
        assert cb.state == "open"
        time.sleep(0.06)
        assert cb.state == "half_open"
        assert cb.allow() is True

    def test_success_resets(self) -> None:
        cb = CircuitBreaker(failure_threshold=3)
        cb.on_failure()
        cb.on_success()
        assert cb.state == "closed"
        assert cb.allow() is True


class TestWithRetry:
    async def test_succeeds_after_retries(self) -> None:
        attempts = {"count": 0}

        @with_retry(
            max_attempts=4,
            initial_delay=0.01,
            max_delay=0.05,
            breaker=CircuitBreaker(failure_threshold=10),
        )
        async def flaky() -> str:
            attempts["count"] += 1
            if attempts["count"] < 4:
                raise TimeoutError("timeout")
            return "ok"

        result = await flaky()
        assert result == "ok"
        assert attempts["count"] == 4

    async def test_4xx_400_not_retried(self) -> None:
        attempts = {"count": 0}

        @with_retry(
            max_attempts=3,
            initial_delay=0.01,
            breaker=CircuitBreaker(failure_threshold=10),
        )
        async def four_hundred() -> str:
            attempts["count"] += 1
            raise Exception("HTTP 400 bad request")

        with pytest.raises(Exception, match="400"):
            await four_hundred()
        assert attempts["count"] == 1

    async def test_circuit_breaker_blocks_when_open(self) -> None:
        cb = CircuitBreaker(failure_threshold=2, recovery_seconds=10)

        @with_retry(max_attempts=1, initial_delay=0.01, breaker=cb)
        async def will_fail() -> str:
            raise RuntimeError("persistent error")

        # 触发 2 次失败 → 熔断打开
        for _ in range(2):
            with pytest.raises(RuntimeError):
                await will_fail()

        # 第 3 次：直接 LLMUnavailableError，不调函数
        with pytest.raises(LLMUnavailableError):
            await will_fail()
