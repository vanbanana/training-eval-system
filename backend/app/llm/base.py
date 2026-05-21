"""LLMProvider 抽象与统一数据模型."""

from __future__ import annotations

from abc import ABC, abstractmethod
from collections.abc import AsyncIterator
from typing import Literal

from pydantic import BaseModel, Field


class LLMMessage(BaseModel):
    role: Literal["system", "user", "assistant", "tool"]
    content: str
    tool_call_id: str | None = None
    name: str | None = None


class ToolCall(BaseModel):
    id: str
    name: str
    arguments: dict[str, object] = Field(default_factory=dict)


class LLMResponse(BaseModel):
    content: str = ""
    model: str = ""
    prompt_tokens: int = 0
    completion_tokens: int = 0
    tool_calls: list[ToolCall] | None = None
    finish_reason: str = ""


class LLMProvider(ABC):
    """LLM Provider 接口（OpenAI 兼容协议）."""

    @abstractmethod
    async def chat(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
        max_tokens: int | None = None,
    ) -> LLMResponse:
        """普通对话调用."""

    @abstractmethod
    async def chat_with_tools(
        self,
        messages: list[LLMMessage],
        tools: list[dict[str, object]],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> LLMResponse:
        """支持 function calling 的对话."""

    @abstractmethod
    async def chat_stream(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> AsyncIterator[str]:
        """SSE 流式输出，yield delta 文本."""

    @abstractmethod
    async def embed(
        self, texts: list[str], *, model: str | None = None
    ) -> list[list[float]]:
        """获取文本向量."""

    @abstractmethod
    async def health_check(self) -> bool:
        """连通性检测."""
