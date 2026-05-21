"""Task 10.4 验收：LLM Metrics 装饰器."""

from __future__ import annotations

import pytest

from app.llm.base import LLMResponse
from app.llm.metrics import get_metrics, reset_metrics, with_metrics


pytestmark = pytest.mark.unit


@pytest.fixture(autouse=True)
def _reset() -> None:
    reset_metrics()
    yield
    reset_metrics()


class TestSuccess:
    async def test_success_increments_counter(self) -> None:
        @with_metrics("chat")
        async def fake() -> LLMResponse:
            return LLMResponse(
                content="ok", prompt_tokens=10, completion_tokens=5
            )

        await fake()
        m = get_metrics()
        assert m["counters"]["llm.call.chat.success"] == 1
        assert m["counters"]["llm.tokens.total"] == 15

    async def test_records_duration(self) -> None:
        @with_metrics("embed")
        async def fake() -> LLMResponse:
            return LLMResponse()

        await fake()
        m = get_metrics()
        assert len(m["durations_ms"]["embed"]) == 1
        assert m["durations_ms"]["embed"][0] >= 0


class TestFailure:
    async def test_failure_increments_failed_counter(self) -> None:
        @with_metrics("chat")
        async def fake() -> LLMResponse:
            raise RuntimeError("boom")

        with pytest.raises(RuntimeError):
            await fake()
        m = get_metrics()
        assert m["counters"]["llm.call.chat.failed"] == 1
        assert "llm.call.chat.success" not in m["counters"]
