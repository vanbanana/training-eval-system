"""认证路由."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.api.deps import CurrentUser, DbSession
from app.schemas.auth import AuthResponse, LoginRequest, UserPublic
from app.services.auth_service import login, refresh_tokens

router = APIRouter(prefix="/api/auth", tags=["auth"])


class RefreshRequest(BaseModel):
    refresh_token: str = Field(..., min_length=10)


@router.post("/login", response_model=AuthResponse)
async def login_endpoint(payload: LoginRequest, db: DbSession) -> AuthResponse:
    return await login(db, username=payload.username, password=payload.password)


@router.post("/refresh", response_model=AuthResponse)
async def refresh_endpoint(payload: RefreshRequest, db: DbSession) -> AuthResponse:
    """用 refresh_token 换新的 access + refresh token 对."""
    return await refresh_tokens(db, refresh_token=payload.refresh_token)


@router.get("/me", response_model=UserPublic)
async def me(current: CurrentUser) -> UserPublic:
    return UserPublic.model_validate(current, from_attributes=True)


@router.post("/logout", status_code=204)
async def logout() -> None:
    """前端清除 token；服务端无状态，不维护黑名单."""
    return None
