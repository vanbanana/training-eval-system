"""用户管理路由（管理员）."""

from __future__ import annotations

from fastapi import APIRouter, Query
from pydantic import BaseModel, Field

from app.api.deps import CurrentUser, DbSession
from app.schemas.auth import UserPublic
from app.services.user_service import user_service

router = APIRouter(prefix="/api/users", tags=["users"])


class CreateUserRequest(BaseModel):
    username: str = Field(..., min_length=2, max_length=64)
    display_name: str = Field(..., min_length=1, max_length=100)
    role: str = Field(..., pattern="^(admin|teacher|student)$")
    password: str = Field(..., min_length=8, max_length=128)


class UpdateUserRequest(BaseModel):
    display_name: str | None = Field(default=None, min_length=1, max_length=100)
    role: str | None = Field(default=None, pattern="^(admin|teacher|student)$")
    is_active: bool | None = None


class ResetPasswordRequest(BaseModel):
    new_password: str = Field(..., min_length=8, max_length=128)


@router.get("", response_model=list[UserPublic])
async def list_users(
    db: DbSession,
    current: CurrentUser,
    role: str | None = Query(default=None, pattern="^(admin|teacher|student)$"),
    offset: int = Query(default=0, ge=0),
    limit: int = Query(default=50, ge=1, le=200),
) -> list[UserPublic]:
    users = await user_service.list_users(
        db, actor=current, role=role, offset=offset, limit=limit
    )
    return [UserPublic.model_validate(u, from_attributes=True) for u in users]


@router.post("", response_model=UserPublic, status_code=201)
async def create_user(
    payload: CreateUserRequest, db: DbSession, current: CurrentUser
) -> UserPublic:
    user = await user_service.create_user(
        db,
        actor=current,
        data={
            "username": payload.username,
            "display_name": payload.display_name,
            "role": payload.role,
            "password": payload.password,
        },
    )
    return UserPublic.model_validate(user, from_attributes=True)


@router.patch("/{user_id}", response_model=UserPublic)
async def update_user(
    user_id: int,
    payload: UpdateUserRequest,
    db: DbSession,
    current: CurrentUser,
) -> UserPublic:
    data: dict[str, object] = payload.model_dump(exclude_unset=True)
    user = await user_service.update_user(
        db,
        actor=current,
        user_id=user_id,
        data=data,  # type: ignore[arg-type]
    )
    return UserPublic.model_validate(user, from_attributes=True)


@router.patch("/{user_id}/toggle-active", response_model=UserPublic)
async def toggle_active(
    user_id: int, db: DbSession, current: CurrentUser
) -> UserPublic:
    """便捷端点：根据当前状态翻转 is_active."""
    target = await user_service.repo.get(db, user_id)
    if target is None:
        from app.core.exceptions import ResourceNotFoundError

        raise ResourceNotFoundError(f"user {user_id} not found")
    new_state = not target.is_active
    user = await user_service.update_user(
        db, actor=current, user_id=user_id, data={"is_active": new_state}
    )
    return UserPublic.model_validate(user, from_attributes=True)


@router.post("/{user_id}/reset-password", status_code=204)
async def reset_password(
    user_id: int,
    payload: ResetPasswordRequest,
    db: DbSession,
    current: CurrentUser,
) -> None:
    await user_service.reset_password(
        db, actor=current, user_id=user_id, new_password=payload.new_password
    )
