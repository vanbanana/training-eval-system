"""FastAPI 依赖项 - 当前用户、角色守卫、DB session."""

from __future__ import annotations

from collections.abc import Callable
from typing import Annotated

from fastapi import Depends
from fastapi.security import OAuth2PasswordBearer
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.database import get_db
from app.core.exceptions import AuthenticationError, AuthorizationError
from app.core.security import decode_token
from app.models.user import User
from app.services.auth_service import get_user_by_id

oauth2_scheme = OAuth2PasswordBearer(tokenUrl="/api/auth/login", auto_error=False)


async def get_current_user(
    token: Annotated[str | None, Depends(oauth2_scheme)],
    db: Annotated[AsyncSession, Depends(get_db)],
) -> User:
    """从 Bearer token 解析出当前用户；token 缺失/非法 → 401."""
    if not token:
        raise AuthenticationError("缺少认证信息")
    payload = decode_token(token)
    try:
        user_id = int(payload["sub"])
    except (KeyError, ValueError) as e:
        raise AuthenticationError("token 格式异常") from e
    user = await get_user_by_id(db, user_id)
    if not user.is_active:
        raise AuthenticationError("账号已被禁用")
    return user


def require_roles(*roles: str) -> Callable[[User], User]:
    """生成一个依赖函数：仅允许指定角色访问。

    使用：
        admin_only = require_roles("admin")

        @router.get("/users", dependencies=[Depends(admin_only)])
        async def ...
    """

    async def _check(user: Annotated[User, Depends(get_current_user)]) -> User:
        if user.role not in roles:
            raise AuthorizationError(f"需要 {'/'.join(roles)} 角色")
        return user

    return _check


# 常用快捷依赖
require_admin = require_roles("admin")
require_teacher = require_roles("teacher", "admin")
require_student = require_roles("student", "teacher", "admin")


CurrentUser = Annotated[User, Depends(get_current_user)]
DbSession = Annotated[AsyncSession, Depends(get_db)]
AdminUser = Annotated[User, Depends(require_admin)]
TeacherUser = Annotated[User, Depends(require_teacher)]
