"""Task 1.10 验收：FastAPI 入口与健康检查."""

from __future__ import annotations

import pytest
from httpx import ASGITransport, AsyncClient


pytestmark = pytest.mark.integration


@pytest.fixture()
async def client() -> AsyncClient:
    from app.main import app

    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        yield c


async def test_healthz_returns_ok(client: AsyncClient) -> None:
    """Given app 已启动；When GET /healthz；Then 200 + status=ok。"""
    r = await client.get("/healthz")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ok"
    assert "env" in body


async def test_healthz_returns_trace_id_header(client: AsyncClient) -> None:
    """Given /healthz 请求；When 检查响应；Then 含 X-Trace-Id 头。"""
    r = await client.get("/healthz")
    assert "X-Trace-Id" in r.headers or "x-trace-id" in r.headers


async def test_swagger_docs_available(client: AsyncClient) -> None:
    """Given env=dev/test；When GET /docs；Then 200 OR 重定向后 200。"""
    r = await client.get("/docs", follow_redirects=True)
    assert r.status_code == 200
    assert "swagger" in r.text.lower() or "openapi" in r.text.lower()


async def test_openapi_schema_endpoint(client: AsyncClient) -> None:
    r = await client.get("/openapi.json")
    assert r.status_code == 200
    schema = r.json()
    assert schema["info"]["title"] == "智能实训评价管理系统 API"


async def test_unknown_route_returns_404_with_standard_format(client: AsyncClient) -> None:
    """Given 不存在的路径；When GET；Then 404 + 标准 JSON 格式。"""
    r = await client.get("/this-does-not-exist")
    assert r.status_code == 404
    body = r.json()
    # 由 _http_exc_handler 输出
    assert "trace_id" in body
