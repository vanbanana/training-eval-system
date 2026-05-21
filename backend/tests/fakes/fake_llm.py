"""FakeLLM - 测试替身，按 messages 内容匹配预设响应."""

from __future__ import annotations

import hashlib
import json
from collections import deque
from collections.abc import AsyncIterator, Callable

from app.llm.base import LLMMessage, LLMProvider, LLMResponse


class FakeLLM(LLMProvider):
    def __init__(
        self,
        default: LLMResponse | None = None,
    ) -> None:
        self._matchers: list[
            tuple[Callable[[list[LLMMessage]], bool], LLMResponse]
        ] = []
        self._default = default or LLMResponse(content="default fake response")
        self._failures: deque[Exception] = deque()
        self.calls: list[list[LLMMessage]] = []

    # ============ 控制接口 ============

    def set_response(
        self,
        matcher: Callable[[list[LLMMessage]], bool],
        response: LLMResponse,
    ) -> None:
        self._matchers.append((matcher, response))

    def set_response_for_text(self, text: str, response: LLMResponse) -> None:
        """匹配最后一条 user 消息含 text."""

        def _m(msgs: list[LLMMessage]) -> bool:
            for m in reversed(msgs):
                if m.role == "user" and text in m.content:
                    return True
            return False

        self.set_response(_m, response)

    def fail_next(self, exception: Exception) -> None:
        self._failures.append(exception)

    def _resolve(self, messages: list[LLMMessage]) -> LLMResponse:
        if self._failures:
            raise self._failures.popleft()
        for m, r in self._matchers:
            if m(messages):
                return r
        return self._default

    @staticmethod
    def hash_messages(messages: list[LLMMessage]) -> str:
        s = json.dumps(
            [{"r": m.role, "c": m.content} for m in messages],
            ensure_ascii=False,
        )
        return hashlib.sha256(s.encode("utf-8")).hexdigest()[:16]

    # ============ Provider 接口 ============

    async def chat(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
        max_tokens: int | None = None,
    ) -> LLMResponse:
        self.calls.append(list(messages))
        return self._resolve(messages)

    async def chat_with_tools(
        self,
        messages: list[LLMMessage],
        tools: list[dict[str, object]],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> LLMResponse:
        self.calls.append(list(messages))
        return self._resolve(messages)

    async def chat_stream(
        self,
        messages: list[LLMMessage],
        *,
        model: str | None = None,
        temperature: float = 0.3,
    ) -> AsyncIterator[str]:
        self.calls.append(list(messages))
        resp = self._resolve(messages)
        for word in resp.content.split():
            yield word + " "

    async def embed(
        self, texts: list[str], *, model: str | None = None
    ) -> list[list[float]]:
        # 确定性 embedding：hash + repeat
        result = []
        for t in texts:
            h = hashlib.sha256(t.encode("utf-8")).digest()
            vec = [b / 255.0 for b in h] + [0.0] * (512 - len(h))
            result.append(vec[:512])
        return result

    async def health_check(self) -> bool:
        return not self._failures
