"""Epic 26.5/26.7 验收：Fake 测试替身."""

from __future__ import annotations

import pytest

from tests.fakes.fake_embedder import FakeEmbedder
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


class TestFakeEmbedder:
    def test_returns_512_dim(self) -> None:
        e = FakeEmbedder()
        v = e.embed(["hello"])
        assert len(v) == 1
        assert len(v[0]) == 512

    def test_deterministic(self) -> None:
        e = FakeEmbedder()
        a = e.embed(["abc"])[0]
        b = e.embed(["abc"])[0]
        assert a == b


class TestFakeLLMStream:
    async def test_stream_yields_tokens(self) -> None:
        from app.llm.base import LLMResponse

        fake = FakeLLM(default=LLMResponse(content="hello world"))
        chunks = []
        async for c in fake.chat_stream([]):  # type: ignore[arg-type]
            chunks.append(c)
        assert chunks
        assert "".join(chunks).strip().startswith("hello")
