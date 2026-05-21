"""Task 1.6 验收：全局异常处理器."""

from __future__ import annotations

import pytest
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from pydantic import BaseModel

from app.core.exceptions import BusinessRuleError
from app.core.exception_handlers import register_exception_handlers
from app.core.middleware import TraceIdMiddleware


pytestmark = pytest.mark.integration


@pytest.fixture()
async def err_app() -> AsyncClient:
    """app 装好 trace + 异常处理；提供 3 个路由触发不同异常。"""
    app = FastAPI()
    app.add_middleware(TraceIdMiddleware)
    register_exception_handlers(app)

    @app.get("/raise/business")
    async def biz() -> None:
        raise BusinessRuleError("权重和为85", field="dimensions")

    @app.get("/raise/unhandled")
    async def boom() -> None:
        raise ZeroDivisionError("x / 0")

    class _Body(BaseModel):
        name: str
        weight: int

    @app.post("/raise/validation")
    async def val(_b: _Body) -> dict[str, str]:
        return {"ok": "true"}

    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        yield c


async def test_business_error_returns_standard_response(err_app: AsyncClient) -> None:
    """Given 路由抛 BusinessRuleError；When 调用；Then 400 + 标准 JSON 含 error_code/field/trace_id。"""
    r = await err_app.get("/raise/business", headers={"X-Trace-Id": "t-biz"})

    assert r.status_code == 400
    body = r.json()
    assert body["error_code"] == "BUSINESS_RULE_VIOLATED"
    assert body["message"] == "权重和为85"
    assert body["field"] == "dimensions"
    assert body["trace_id"] == "t-biz"


async def test_unhandled_exception_returns_500_safe_message(err_app: AsyncClient) -> None:
    """Given 路由抛 ZeroDivisionError；When 调用；
    Then 500 + INTERNAL_ERROR + 不含原始 stacktrace。"""
    r = await err_app.get("/raise/unhandled", headers={"X-Trace-Id": "t-bad"})

    assert r.status_code == 500
    body = r.json()
    assert body["error_code"] == "INTERNAL_ERROR"
    # 不能把 ZeroDivisionError 字面或 traceback 暴露给客户端
    assert "ZeroDivisionError" not in body.get("message", "")
    assert "Traceback" not in str(body)
    assert body["trace_id"] == "t-bad"


async def test_validation_error_returns_422_with_details(err_app: AsyncClient) -> None:
    """Given 客户端发非法 body；When POST；Then 422 + details 含 loc/msg/type。"""
    r = await err_app.post(
        "/raise/validation",
        json={"name": 1},  # weight 缺失，name 类型错
    )

    assert r.status_code == 422
    body = r.json()
    assert body["error_code"] == "VALIDATION_FAILED"
    assert isinstance(body["details"], list)
    assert len(body["details"]) >= 1
    first = body["details"][0]
    assert "loc" in first
    assert "msg" in first
    assert "type" in first


async def test_business_error_no_field_returns_null(err_app: AsyncClient) -> None:
    """Given BusinessRuleError 不带 field；When 触发；Then field 为 null。"""
    app = FastAPI()
    register_exception_handlers(app)

    @app.get("/no-field")
    async def x() -> None:
        raise BusinessRuleError("oops")  # 不传 field

    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://t") as c:
        r = await c.get("/no-field")

    assert r.status_code == 400
    assert r.json()["field"] is None
