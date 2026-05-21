"""个人账号设置路由."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import BusinessRuleError
from app.core.security import hash_password, verify_password

router = APIRouter(prefix="/api/account", tags=["account"])


@router.get("/me")
async def get_account(current: CurrentUser) -> dict[str, object]:
    return {
        "id": current.id,
        "username": current.username,
        "display_name": current.display_name,
        "role": current.role,
        "is_active": current.is_active,
        "last_login_at": current.last_login_at.isoformat() if current.last_login_at else None,
        "created_at": current.created_at.isoformat(),
    }


class UpdateProfileRequest(BaseModel):
    display_name: str = Field(..., min_length=1, max_length=100)


@router.patch("/profile")
async def update_profile(payload: UpdateProfileRequest, db: DbSession, current: CurrentUser) -> dict[str, str]:
    current.display_name = payload.display_name
    await db.commit()
    return {"status": "ok", "display_name": current.display_name}


class ChangePasswordRequest(BaseModel):
    old_password: str = Field(..., min_length=1)
    new_password: str = Field(..., min_length=8, max_length=128)


@router.post("/change-password")
async def change_password(payload: ChangePasswordRequest, db: DbSession, current: CurrentUser) -> dict[str, str]:
    if not verify_password(payload.old_password, current.password_hash):
        raise BusinessRuleError("旧密码错误", field="old_password")
    current.password_hash = hash_password(payload.new_password)
    await db.commit()
    return {"status": "ok"}
