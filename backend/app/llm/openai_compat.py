"""OpenAI 兼容协议 Provider - 真实实现.

通过 httpx + openai SDK 调用任意 OpenAI 兼容 API（通义/DeepSeek/智谱/Moonshot）。
支持：chat / chat_with_tools / chat_stream / embed / health_check。
"""

from __future__ import annotations

import json
from collections.abc import AsyncIterator
from typing import Any

import httpx

from app.core.logging import get_logger
from app.llm.base import LLMMessage, LLMProvider, LLMResponse, ToolCall
from app.llm.retry import with_retry


log = get_logger(__name__)


class OpenAICompatProvider(LLMProvider):
    """OpenAI 兼容协议 Provider（通义/DeepSeek/智谱/Moonshot 通用）."""

    def __init__(
        self,
        *,
        base_url: str,
        api_key: str,
        model: str,
        embed_model: str = "",
        timeout: float = 60,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.embed_model = embed_model or model
        self.timeout = timeout
        self._client: httpx.AsyncClient | None = None

    def _get_client(self) -> httpx.AsyncClient:
        if self._client is None or self._client.is_closed:
            self._client = httpx.AsyncClient(
                base_url=self.base_url,
                headers={
                    "Authorization": f"Bearer {self.api_key}",
                    "Content-Type": "application/json",
                },
                timeout=httpx.Timeout(self.timeout, connect=10.0),
            )
        return self._client

    def _build_messages(self, messages: list[LLMMessage]) -> list[dict[str, Any]]:
        """将内部消息格式转为 OpenAI API 格式."""
        result: list[dict[str, Any]] = []
        for msg in messages:
            item: dict[str, Any] = {"role": msg.role, "content": msg.content}
            if msg.tool_call_id:
                item["tool_call_id"] = msg.tool_call_id
            if msg.name:
                item["name"] = msg.name
            result.append(item)
        return result

    @with_retry(max_attempts=3, initial_delay=1.0, max_delay=10.0)
    async def chat(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
        max_tokens: int | None = None,
    ) -> LLMResponse:
        """普通对话调用."""
        client = self._get_client()
        payload: dict[str, Any] = {
            "model": model or self.model,
            "messages": self._build_messages(messages),
            "temperature": temperature,
        }
        if max_tokens is not None:
            payload["max_tokens"] = max_tokens

        log.info(
            "llm.chat.request",
            model=payload["model"],
            msg_count=len(messages),
        )

        resp = await client.post("/chat/completions", json=payload)
        resp.raise_for_status()
        data = resp.json()

        choice = data["choices"][0]
        message = choice["message"]
        usage = data.get("usage", {})

        return LLMResponse(
            content=message.get("content") or "",
            model=data.get("model", payload["model"]),
            prompt_tokens=usage.get("prompt_tokens", 0),
            completion_tokens=usage.get("completion_tokens", 0),
            finish_reason=choice.get("finish_reason", "stop"),
        )

    @with_retry(max_attempts=3, initial_delay=1.0, max_delay=10.0)
    async def chat_with_tools(
        self,
        messages: list[LLMMessage],
        tools: list[dict[str, object]],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> LLMResponse:
        """支持 Function Calling 的对话."""
        client = self._get_client()
        payload: dict[str, Any] = {
            "model": model or self.model,
            "messages": self._build_messages(messages),
            "temperature": temperature,
            "tools": tools,
        }

        log.info(
            "llm.chat_with_tools.request",
            model=payload["model"],
            tool_count=len(tools),
        )

        resp = await client.post("/chat/completions", json=payload)
        resp.raise_for_status()
        data = resp.json()

        choice = data["choices"][0]
        message = choice["message"]
        usage = data.get("usage", {})

        # 解析 tool_calls
        tool_calls: list[ToolCall] | None = None
        raw_tool_calls = message.get("tool_calls")
        if raw_tool_calls:
            tool_calls = []
            for tc in raw_tool_calls:
                func = tc.get("function", {})
                args_str = func.get("arguments", "{}")
                try:
                    args = json.loads(args_str) if isinstance(args_str, str) else args_str
                except json.JSONDecodeError:
                    args = {}
                tool_calls.append(
                    ToolCall(
                        id=tc.get("id", ""),
                        name=func.get("name", ""),
                        arguments=args,
                    )
                )

        return LLMResponse(
            content=message.get("content") or "",
            model=data.get("model", payload["model"]),
            prompt_tokens=usage.get("prompt_tokens", 0),
            completion_tokens=usage.get("completion_tokens", 0),
            tool_calls=tool_calls,
            finish_reason=choice.get("finish_reason", "stop"),
        )

    async def chat_stream(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> AsyncIterator[str]:
        """SSE 流式输出，yield delta 文本."""
        client = self._get_client()
        payload: dict[str, Any] = {
            "model": model or self.model,
            "messages": self._build_messages(messages),
            "temperature": temperature,
            "stream": True,
        }

        log.info("llm.chat_stream.request", model=payload["model"])

        async with client.stream("POST", "/chat/completions", json=payload) as resp:
            resp.raise_for_status()
            async for line in resp.aiter_lines():
                if not line.startswith("data: "):
                    continue
                data_str = line[6:].strip()
                if data_str == "[DONE]":
                    break
                try:
                    chunk = json.loads(data_str)
                    delta = chunk["choices"][0].get("delta", {})
                    content = delta.get("content")
                    if content:
                        yield content
                except (json.JSONDecodeError, KeyError, IndexError):
                    continue

    @with_retry(max_attempts=3, initial_delay=1.0, max_delay=10.0)
    async def embed(
        self, texts: list[str], *, model: str | None = None
    ) -> list[list[float]]:
        """获取文本向量."""
        client = self._get_client()
        embed_model = model or self.embed_model
        payload: dict[str, Any] = {
            "model": embed_model,
            "input": texts,
        }

        log.info("llm.embed.request", model=embed_model, text_count=len(texts))

        resp = await client.post("/embeddings", json=payload)
        resp.raise_for_status()
        data = resp.json()

        embeddings: list[list[float]] = []
        for item in sorted(data["data"], key=lambda x: x["index"]):
            embeddings.append(item["embedding"])
        return embeddings

    async def health_check(self) -> bool:
        """连通性检测：发送一个极简请求."""
        try:
            client = self._get_client()
            resp = await client.post(
                "/chat/completions",
                json={
                    "model": self.model,
                    "messages": [{"role": "user", "content": "hi"}],
                    "max_tokens": 5,
                },
            )
            return resp.status_code == 200
        except Exception as e:  # noqa: BLE001
            log.warning("llm.health_check.failed", error=str(e))
            return False

    async def close(self) -> None:
        """关闭 HTTP 客户端."""
        if self._client and not self._client.is_closed:
            await self._client.aclose()
            self._client = None
