"""UserFactory - 测试用户工厂.

使用：
    user = await UserFactory.create_async(session, role="teacher")
"""

from __future__ import annotations

from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.security import hash_password
from app.models.user import User
from tests.factories import faker


class _BaseUserFactory:
    """轻量 factory（不用 factory_boy 的 SQLAlchemyModelFactory，避免版本兼容问题）."""

    role = "student"

    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        username: str | None = None,
        display_name: str | None = None,
        role: str | None = None,
        password: str = "Pa$$w0rd2024",
        password_hash: str | None = None,
        is_active: bool = True,
        **extra: Any,
    ) -> User:
        username = username or f"u_{faker.uuid4()[:8]}"
        display_name = display_name or faker.name()
        user = User(
            username=username,
            display_name=display_name,
            password_hash=password_hash or hash_password(password),
            role=role or cls.role,
            is_active=is_active,
            **extra,
        )
        session.add(user)
        await session.flush()
        await session.refresh(user)
        return user


class UserFactory(_BaseUserFactory):
    role = "student"


class TeacherFactory(_BaseUserFactory):
    role = "teacher"


class AdminFactory(_BaseUserFactory):
    role = "admin"
