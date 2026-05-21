"""全局异常 → 统一 JSON 响应."""

from __future__ import annotations

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from app.core.exceptions import BusinessError
from app.core.logging import get_logger, trace_id_ctx

log = get_logger(__name__)


def _trace_id(req: Request) -> str:
    """从 contextvars 或 request.state 读 trace_id（双写双读）."""
    via_ctx = trace_id_ctx.get()
    if via_ctx:
        return via_ctx
    return getattr(req.state, "trace_id", "") or ""


async def _business_handler(req: Request, exc: BusinessError) -> JSONResponse:
    return JSONResponse(
        status_code=exc.http_status,
        content={
            "error_code": exc.error_code,
            "message": exc.message,
            "field": exc.field,
            "trace_id": _trace_id(req),
        },
    )


async def _validation_handler(req: Request, exc: RequestValidationError) -> JSONResponse:
    details = []
    for err in exc.errors():
        details.append({
            "loc": list(err.get("loc", [])),
            "msg": err.get("msg", ""),
            "type": err.get("type", ""),
        })
    return JSONResponse(
        status_code=422,
        content={
            "error_code": "VALIDATION_FAILED",
            "message": "请求参数校验失败",
            "details": details,
            "trace_id": _trace_id(req),
        },
    )


async def _http_exc_handler(req: Request, exc: StarletteHTTPException) -> JSONResponse:
    return JSONResponse(
        status_code=exc.status_code,
        content={
            "error_code": "HTTP_ERROR",
            "message": exc.detail or "",
            "trace_id": _trace_id(req),
        },
    )


async def _unhandled_handler(req: Request, exc: Exception) -> JSONResponse:
    log.exception("unhandled.error", error_type=type(exc).__name__)
    return JSONResponse(
        status_code=500,
        content={
            "error_code": "INTERNAL_ERROR",
            "message": "服务暂时不可用，请稍后重试",
            "trace_id": _trace_id(req),
        },
    )


def register_exception_handlers(app: FastAPI) -> None:
    """统一 JSON 错误响应；含业务异常 + 校验错误 + HTTP 错误 + 未捕获兜底."""
    app.add_exception_handler(BusinessError, _business_handler)  # type: ignore[arg-type]
    app.add_exception_handler(RequestValidationError, _validation_handler)  # type: ignore[arg-type]
    app.add_exception_handler(StarletteHTTPException, _http_exc_handler)  # type: ignore[arg-type]
    app.add_exception_handler(Exception, _unhandled_handler)
