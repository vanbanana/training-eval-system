"""Task 1.5 验收：TraceIdMiddleware + contextvars."""

from __future__ import annotations

import re

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient

from app.core.logging import trace_id_ctx
from app.core.middleware import TraceIdMiddleware


pytestmark = pytest.mark.integration


@pytest.fixture()
async def trace_app() -> AsyncClient:
    """轻量 app 仅装 TraceIdMiddleware，不依赖 main.py 完整 wiring."""
    app = FastAPI()
    app.add_middleware(TraceIdMiddleware)

    captured: dict[str, str] = {}

    @app.get("/probe")
    async def probe() -> dict[str, str]:
        captured["trace_id"] = trace_id_ctx.get()
        return {"trace_id": trace_id_ctx.get()}

    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://t") as client:
        client._captured = captured  # type: ignore[attr-defined]
        yield client


async def test_trace_id_passthrough_from_request_header(trace_app: AsyncClient) -> None:
    """Given 客户端发 X-Trace-Id；When 服务端处理；Then 响应头与上下文相同。"""
    r = await trace_app.get("/probe", headers={"X-Trace-Id": "my-trace-001"})

    assert r.status_code == 200
    assert r.headers["X-Trace-Id"] == "my-trace-001"
    assert r.json()["trace_id"] == "my-trace-001"


async def test_trace_id_generated_when_missing(trace_app: AsyncClient) -> None:
    """Given 客户端无 trace 头；When 服务端处理；Then 响应头是 UUID4 格式。"""
    r = await trace_app.get("/probe")

    assert r.status_code == 200
    trace_id = r.headers["X-Trace-Id"]
    uuid4_pattern = (
        r"^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$"
    )
    assert re.match(uuid4_pattern, trace_id), f"{trace_id} 不是 UUID4"


async def test_oversized_trace_id_truncated_to_64(trace_app: AsyncClient) -> None:
    """Given 客户端发长度 300 的 trace；When 服务端处理；Then 响应头长度 ≤ 64 不抛异常。"""
    long_trace = "a" * 300
    r = await trace_app.get("/probe", headers={"X-Trace-Id": long_trace})

    assert r.status_code == 200
    assert len(r.headers["X-Trace-Id"]) <= 64


async def test_context_cleared_after_request(trace_app: AsyncClient) -> None:
    """Given 一次请求；When 完成；Then 主线程 context 被还原（无污染下次请求）。"""
    await trace_app.get("/probe", headers={"X-Trace-Id": "leak-check"})
    # ASGI 模式 context 在请求结束 finally 中清除
    assert trace_id_ctx.get() == ""
