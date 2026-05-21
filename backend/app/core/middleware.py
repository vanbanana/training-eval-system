"""trace_id 中间件 - 纯 ASGI 实现，trace_id 同时写入 contextvars 与 request.state.

为什么两处都写：
- contextvars：日志/异常等无 Request 上下文的位置可读
- request.state：Starlette 的 Exception 处理器在更外层，contextvars 已被 reset，
  但 request.state 仍随 Request 实例可用
"""

from __future__ import annotations

import uuid

from starlette.types import ASGIApp, Message, Receive, Scope, Send

from app.core.logging import trace_id_ctx, user_id_ctx


class TraceIdMiddleware:
    """从 X-Trace-Id 请求头读 trace；缺失则生成 UUID4；响应头回写。"""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        headers = dict(scope.get("headers", []))
        raw = headers.get(b"x-trace-id", b"").decode("latin-1")
        trace_id = raw or str(uuid.uuid4())
        if len(trace_id) > 64:
            trace_id = trace_id[:64]

        # 写入 scope.state，让 Exception 处理器在更外层仍可读
        scope.setdefault("state", {})
        scope["state"]["trace_id"] = trace_id  # type: ignore[index]

        token_t = trace_id_ctx.set(trace_id)
        token_u = user_id_ctx.set(None)

        async def send_with_trace(message: Message) -> None:
            if message["type"] == "http.response.start":
                response_headers = list(message.get("headers", []))
                response_headers = [
                    (k, v) for k, v in response_headers if k.lower() != b"x-trace-id"
                ]
                response_headers.append((b"x-trace-id", trace_id.encode("latin-1")))
                message["headers"] = response_headers
            await send(message)

        try:
            await self.app(scope, receive, send_with_trace)
        finally:
            trace_id_ctx.reset(token_t)
            user_id_ctx.reset(token_u)


def get_current_trace_id() -> str:
    """获取当前请求的 trace_id（仅在中间件作用域内有效）."""
    return trace_id_ctx.get()


__all__ = ["TraceIdMiddleware", "get_current_trace_id"]
