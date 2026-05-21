"""Task 10.1 + 10.8 验收：LLMProvider 接口 + FakeLLM."""

from __future__ import annotations

import pytest

from app.core.exceptions import LLMUnavailableError
from app.llm.base import LLMMessage, LLMProvider, LLMResponse, ToolCall
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


class TestSchemas:
    def test_message_default_fields(self) -> None:
        m = LLMMessage(role="user", content="hi")
        assert m.tool_call_id is None
        assert m.name is None

    def test_response_default_zero_tokens(self) -> None:
        r = LLMResponse()
        assert r.prompt_tokens == 0
        assert r.completion_tokens == 0

    def test_tool_call_arguments_default_dict(self) -> None:
        tc = ToolCall(id="t1", name="search")
        assert tc.arguments == {}


class TestAbstractClass:
    def test_cannot_instantiate_abstract(self) -> None:
        with pytest.raises(TypeError):
            LLMProvider()  # type: ignore[abstract]


class TestFakeLLMHappyPath:
    async def test_default_response(self) -> None:
        fake = FakeLLM()
        result = await fake.chat([LLMMessage(role="user", content="hi")])
        assert result.content == "default fake response"

    async def test_match_by_text(self) -> None:
        fake = FakeLLM()
        fake.set_response_for_text(
            "score", LLMResponse(content="评分结果")
        )
        result = await fake.chat([LLMMessage(role="user", content="please score this")])
        assert result.content == "评分结果"

    async def test_records_calls(self) -> None:
        fake = FakeLLM()
        await fake.chat([LLMMessage(role="user", content="x")])
        await fake.chat([LLMMessage(role="user", content="y")])
        assert len(fake.calls) == 2


class TestFakeLLMFailures:
    async def test_fail_next_consumed_once(self) -> None:
        fake = FakeLLM()
        fake.fail_next(LLMUnavailableError("simulated"))
        with pytest.raises(LLMUnavailableError):
            await fake.chat([LLMMessage(role="user", content="x")])
        # 二次调用恢复正常
        result = await fake.chat([LLMMessage(role="user", content="x")])
        assert result.content


class TestFakeLLMEmbedAndStream:
    async def test_embed_returns_512_dim_vectors(self) -> None:
        fake = FakeLLM()
        vecs = await fake.embed(["a", "b", "c"])
        assert len(vecs) == 3
        assert all(len(v) == 512 for v in vecs)

    async def test_embed_deterministic(self) -> None:
        fake = FakeLLM()
        v1 = await fake.embed(["same input"])
        v2 = await fake.embed(["same input"])
        assert v1 == v2

    async def test_stream_yields_chunks(self) -> None:
        fake = FakeLLM(default=LLMResponse(content="hello world from fake"))
        out = ""
        async for chunk in fake.chat_stream([LLMMessage(role="user", content="x")]):
            out += chunk
        assert "hello" in out
        assert "fake" in out


class TestHealthCheck:
    async def test_returns_true_when_no_failures(self) -> None:
        fake = FakeLLM()
        assert await fake.health_check() is True

    async def test_returns_false_when_failures_pending(self) -> None:
        fake = FakeLLM()
        fake.fail_next(LLMUnavailableError("x"))
        assert await fake.health_check() is False
