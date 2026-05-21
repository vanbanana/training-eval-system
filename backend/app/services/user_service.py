"""UserService - 用户管理业务编排（仅管理员可用）."""

from __future__ import annotations

from typing import TypedDict

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import (
    AuthorizationError,
    ConflictError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.core.security import hash_password
from app.models.user import User
from app.repositories.user_repo import UserRepository

log = get_logger(__name__)


class CreateUserData(TypedDict):
    username: str
    display_name: str
    role: str
    password: str


class UpdateUserData(TypedDict, total=False):
    display_name: str
    role: str
    is_active: bool


class UserService:
    """所有方法接受 actor: User 作为操作者，用于权限校验 + 审计."""

    def __init__(self, repo: UserRepository | None = None) -> None:
        self.repo = repo or UserRepository()

    @staticmethod
    def _ensure_admin(actor: User) -> None:
        if actor.role != "admin":
            raise AuthorizationError("仅管理员可执行此操作")

    async def create_user(
        self,
        db: AsyncSession,
        *,
        actor: User,
        data: CreateUserData,
    ) -> User:
        self._ensure_admin(actor)
        if data["role"] not in {"admin", "teacher", "student"}:
            raise ConflictError(
                f"非法角色 {data['role']}", field="role"
            )

        existing = await self.repo.get_by_username(db, data["username"])
        if existing is not None:
            raise ConflictError(f"用户名 {data['username']} 已存在", field="username")

        user = await self.repo.create(
            db,
            username=data["username"],
            display_name=data["display_name"],
            password_hash=hash_password(data["password"]),
            role=data["role"],
            is_active=True,
        )
        log.info(
            "user.created",
            target_user_id=user.id,
            target_role=user.role,
            actor_id=actor.id,
        )
        return user

    async def update_user(
        self,
        db: AsyncSession,
        *,
        actor: User,
        user_id: int,
        data: UpdateUserData,
    ) -> User:
        self._ensure_admin(actor)
        target = await self.repo.get(db, user_id)
        if target is None:
            raise ResourceNotFoundError(f"user {user_id} not found")

        if "role" in data and data["role"] not in {"admin", "teacher", "student"}:
            raise ConflictError(f"非法角色 {data['role']}", field="role")

        update_fields: dict[str, object] = {}
        if "display_name" in data:
            update_fields["display_name"] = data["display_name"]
        if "role" in data:
            update_fields["role"] = data["role"]
        if "is_active" in data:
            update_fields["is_active"] = data["is_active"]

        if update_fields:
            await self.repo.update(db, user_id, **update_fields)
            await db.refresh(target)

        log.info(
            "user.updated",
            target_user_id=user_id,
            actor_id=actor.id,
            fields=list(update_fields.keys()),
        )
        return target

    async def reset_password(
        self,
        db: AsyncSession,
        *,
        actor: User,
        user_id: int,
        new_password: str,
    ) -> None:
        self._ensure_admin(actor)
        target = await self.repo.get(db, user_id)
        if target is None:
            raise ResourceNotFoundError(f"user {user_id} not found")
        await self.repo.update(
            db, user_id, password_hash=hash_password(new_password)
        )
        log.info("user.password_reset", target_user_id=user_id, actor_id=actor.id)

    async def deactivate_user(
        self,
        db: AsyncSession,
        *,
        actor: User,
        user_id: int,
    ) -> User:
        return await self.update_user(
            db, actor=actor, user_id=user_id, data={"is_active": False}
        )

    async def activate_user(
        self,
        db: AsyncSession,
        *,
        actor: User,
        user_id: int,
    ) -> User:
        return await self.update_user(
            db, actor=actor, user_id=user_id, data={"is_active": True}
        )

    async def list_users(
        self,
        db: AsyncSession,
        *,
        actor: User,
        role: str | None = None,
        offset: int = 0,
        limit: int = 50,
    ) -> list[User]:
        self._ensure_admin(actor)
        if role is not None:
            return await self.repo.list_by_role(
                db, role, offset=offset, limit=limit
            )
        return await self.repo.list(db, offset=offset, limit=limit)


# 模块级单例（无状态，可共享）
user_service = UserService()
