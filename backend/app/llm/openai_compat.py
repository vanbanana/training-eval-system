"""OpenAI 兼容协议 Provider - 占位实现.

完整实现需 httpx + SSE 解析；目前提供 stub，运行时可替换。
"""

from __future__ import annotations

from collections.abc import AsyncIterator

from app.core.exceptions import LLMUnavailableError
from app.llm.base import LLMMessage, LLMProvider, LLMResponse


class OpenAICompatProvider(LLMProvider):
    def __init__(
        self,
        *,
        base_url: str,
        api_key: str,
        model: str,
        timeout: float = 30,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.timeout = timeout

    async def chat(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
        max_tokens: int | None = None,
    ) -> LLMResponse:
        # 实际使用 httpx + openai SDK；此处为 placeholder
        raise LLMUnavailableError(
            "OpenAICompatProvider 占位实现：请联网测试或注入 FakeLLM",
            field="llm",
        )

    async def chat_with_tools(
        self,
        messages: list[LLMMessage],
        tools: list[dict[str, object]],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> LLMResponse:
        raise LLMUnavailableError("OpenAICompatProvider 占位实现", field="llm")

    async def chat_stream(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> AsyncIterator[str]:
        raise LLMUnavailableError("OpenAICompatProvider 占位实现", field="llm")
        yield ""  # pragma: no cover

    async def embed(
        self, texts: list[str], *, model: str | None = None
    ) -> list[list[float]]:
        raise LLMUnavailableError("OpenAICompatProvider 占位实现", field="llm")

    async def health_check(self) -> bool:
        return False
