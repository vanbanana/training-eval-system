"""认证相关 Pydantic schemas."""

from __future__ import annotations

from pydantic import BaseModel, Field


class LoginRequest(BaseModel):
    username: str = Field(..., min_length=1, max_length=64)
    password: str = Field(..., min_length=1, max_length=128)


class UserPublic(BaseModel):
    id: int
    username: str
    display_name: str
    role: str
    is_active: bool


class AuthResponse(BaseModel):
    access_token: str
    refresh_token: str = ""
    token_type: str = "bearer"
    user: UserPublic
