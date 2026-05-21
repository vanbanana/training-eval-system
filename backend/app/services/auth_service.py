"""登录业务编排."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import (
    AccountLockedError,
    AuthenticationError,
    InvalidCredentialsError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.core.security import (
    create_access_token,
    create_refresh_token,
    decode_token,
    verify_password,
)
from app.models.user import User
from app.schemas.auth import AuthResponse, UserPublic

log = get_logger(__name__)

MAX_FAILED = 5
LOCK_MINUTES = 15


async def login(db: AsyncSession, *, username: str, password: str) -> AuthResponse:
    user = (
        await db.execute(select(User).where(User.username == username))
    ).scalar_one_or_none()
    if user is None:
        log.info("auth.login.unknown_user", username=username)
        raise InvalidCredentialsError("账号或密码错误")

    now = datetime.now(UTC)
    if user.locked_until and user.locked_until > now:
        log.warning("auth.login.locked", user_id=user.id, until=user.locked_until.isoformat())
        raise AccountLockedError(f"账号已锁定，请于 {user.locked_until:%H:%M} 后重试")

    if not user.is_active:
        raise InvalidCredentialsError("账号已被禁用")

    if not verify_password(password, user.password_hash):
        user.failed_login_count += 1
        if user.failed_login_count >= MAX_FAILED:
            user.locked_until = now + timedelta(minutes=LOCK_MINUTES)
            log.warning("auth.login.lock_triggered", user_id=user.id)
        await db.commit()
        raise InvalidCredentialsError("账号或密码错误")

    user.failed_login_count = 0
    user.locked_until = None
    user.last_login_at = now
    await db.commit()

    token = create_access_token(user_id=user.id, role=user.role)
    refresh = create_refresh_token(user_id=user.id)
    log.info("auth.login.success", user_id=user.id, role=user.role)
    return AuthResponse(
        access_token=token,
        refresh_token=refresh,
        user=UserPublic.model_validate(user, from_attributes=True),
    )


async def get_user_by_id(db: AsyncSession, user_id: int) -> User:
    user = await db.get(User, user_id)
    if user is None:
        raise ResourceNotFoundError(f"user {user_id} not found")
    return user



class InvalidTokenTypeError(AuthenticationError):
    """传入的 token 不是 refresh 类型."""

    error_code = "INVALID_TOKEN_TYPE"


class UserDisabledError(AuthenticationError):
    error_code = "USER_DISABLED"


async def refresh_tokens(db: AsyncSession, *, refresh_token: str) -> AuthResponse:
    """基于 refresh token 颁发新的 access + refresh token.

    校验：
    1. JWT 合法且未过期（由 decode_token 校验）
    2. type == 'refresh'（防止用 access 当 refresh）
    3. 用户存在且 is_active
    """
    payload = decode_token(refresh_token)
    if payload.get("type") != "refresh":
        raise InvalidTokenTypeError("token type 必须为 refresh")

    sub = payload.get("sub")
    try:
        user_id = int(sub) if sub is not None else None
    except (TypeError, ValueError):
        user_id = None
    if user_id is None:
        raise AuthenticationError("token sub 非法")

    user = await db.get(User, user_id)
    if user is None:
        raise AuthenticationError("用户不存在")
    if not user.is_active:
        raise UserDisabledError("账号已被禁用")

    access = create_access_token(user_id=user.id, role=user.role)
    refresh = create_refresh_token(user_id=user.id)
    log.info("auth.refresh.success", user_id=user.id)
    return AuthResponse(
        access_token=access,
        refresh_token=refresh,
        user=UserPublic.model_validate(user, from_attributes=True),
    )
