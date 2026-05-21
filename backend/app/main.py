"""FastAPI 应用入口."""

from __future__ import annotations

from collections.abc import AsyncIterator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.account import router as account_router
from app.api.audit import router as audit_router
from app.api.auth import router as auth_router
from app.api.chat import router as chat_router
from app.api.classes import router as classes_router
from app.api.courses import router as courses_router
from app.api.dashboard import router as dashboard_router
from app.api.evaluations import router as evaluations_router
from app.api.grading import router as grading_router
from app.api.import_users import router as import_router
from app.api.imports import router as imports_router
from app.api.llm import router as llm_router
from app.api.notifications import router as notifications_router
from app.api.parse import router as parse_router
from app.api.profile import router as profile_router
from app.api.profiles import router as profiles_router
from app.api.reports import router as reports_router
from app.api.similarity import router as similarity_router
from app.api.task_edit import router as task_edit_router
from app.api.task_manage import router as task_manage_router
from app.api.tasks import router as tasks_router
from app.api.templates import router as templates_router
from app.api.uploads import router as uploads_router
from app.api.users import router as users_router
from app.api.websockets import router as websocket_router
from app.core.config import get_settings
from app.core.database import Base, engine
from app.core.exception_handlers import register_exception_handlers
from app.core.logging import configure_logging, get_logger
from app.core.middleware import TraceIdMiddleware

settings = get_settings()
configure_logging(env=settings.env)
log = get_logger(__name__)


@asynccontextmanager
async def lifespan(_: FastAPI) -> AsyncIterator[None]:
    log.info("app.startup", env=settings.env, db=settings.db_url.split("@")[-1])
    # dev 模式自动建表（生产用 alembic）
    if settings.env in {"dev", "test"}:
        async with engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)
    yield
    log.info("app.shutdown")


app = FastAPI(
    title="智能实训评价管理系统 API",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(TraceIdMiddleware)
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

register_exception_handlers(app)

app.include_router(auth_router)
app.include_router(tasks_router)
app.include_router(task_manage_router)
app.include_router(uploads_router)
app.include_router(evaluations_router)
app.include_router(grading_router)
app.include_router(users_router)
app.include_router(courses_router)
app.include_router(classes_router)
app.include_router(llm_router)
app.include_router(audit_router)
app.include_router(profile_router)
app.include_router(profiles_router)
app.include_router(reports_router)
app.include_router(notifications_router)
app.include_router(account_router)
app.include_router(dashboard_router)
app.include_router(templates_router)
app.include_router(similarity_router)
app.include_router(chat_router)
app.include_router(import_router)
app.include_router(imports_router)
app.include_router(task_edit_router)
app.include_router(parse_router)
app.include_router(websocket_router)

# Dev 端点仅在非 prod 启用
if settings.env != "prod":
    from app.api._dev import router as _dev_router

    app.include_router(_dev_router)


@app.get("/healthz")
async def healthz() -> dict[str, str]:
    return {"status": "ok", "env": settings.env}
