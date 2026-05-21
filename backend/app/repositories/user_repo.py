"""UserRepository - 用户数据访问."""

from __future__ import annotations

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.user import User
from app.repositories.base import BaseRepository


class UserRepository(BaseRepository[User]):
    model = User

    async def get_by_username(self, session: AsyncSession, username: str) -> User | None:
        """大小写不敏感地按用户名查询."""
        stmt = select(User).where(func.lower(User.username) == username.lower())
        return (await session.execute(stmt)).scalar_one_or_none()

    async def list_by_role(
        self, session: AsyncSession, role: str, *, offset: int = 0, limit: int = 50
    ) -> list[User]:
        stmt = (
            select(User)
            .where(User.role == role)
            .order_by(User.id)
            .offset(offset)
            .limit(limit)
        )
        return list((await session.execute(stmt)).scalars().all())

    async def list_active(
        self, session: AsyncSession, *, offset: int = 0, limit: int = 50
    ) -> list[User]:
        stmt = (
            select(User)
            .where(User.is_active.is_(True))
            .order_by(User.id)
            .offset(offset)
            .limit(limit)
        )
        return list((await session.execute(stmt)).scalars().all())
