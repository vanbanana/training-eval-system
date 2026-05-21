"""通用 Repository 基类 - 提供 CRUD 操作.

约定：
- 子类指定 model 类
- 所有方法接受 AsyncSession 显式注入（不依赖全局）
- 不包含业务逻辑，仅数据访问
"""

from __future__ import annotations

from typing import Any, ClassVar, Generic, TypeVar

from sqlalchemy import delete, func, select, update
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.database import Base

T = TypeVar("T", bound=Base)


class BaseRepository(Generic[T]):
    """通用 CRUD Repository.

    子类用法：

        class UserRepo(BaseRepository[User]):
            model = User

        repo = UserRepo()
        user = await repo.get(db, 1)
    """

    model: ClassVar[type[Base]]

    async def get(self, session: AsyncSession, id_: int) -> T | None:
        """按主键查询；不存在返回 None。"""
        result: T | None = await session.get(self.model, id_)  # type: ignore[assignment]
        return result

    async def list(
        self,
        session: AsyncSession,
        *,
        offset: int = 0,
        limit: int = 50,
        order_by: Any = None,
    ) -> list[T]:
        stmt = select(self.model).offset(offset).limit(limit)
        if order_by is not None:
            stmt = stmt.order_by(order_by)
        else:
            # 默认按 id 升序
            stmt = stmt.order_by(self.model.__table__.c.id)  # type: ignore[attr-defined]
        result = await session.execute(stmt)
        return list(result.scalars().all())

    async def count(self, session: AsyncSession) -> int:
        stmt = select(func.count()).select_from(self.model)
        result = await session.execute(stmt)
        return int(result.scalar_one())

    async def create(self, session: AsyncSession, **fields: Any) -> T:
        """创建并 flush；commit 由调用方负责。"""
        instance = self.model(**fields)  # type: ignore[call-arg]
        session.add(instance)
        await session.flush()
        await session.refresh(instance)
        return instance  # type: ignore[return-value]

    async def update(
        self,
        session: AsyncSession,
        id_: int,
        **fields: Any,
    ) -> int:
        """按 id 更新；返回 affected rows。"""
        if not fields:
            return 0
        stmt = (
            update(self.model)
            .where(self.model.__table__.c.id == id_)  # type: ignore[attr-defined]
            .values(**fields)
        )
        result = await session.execute(stmt)
        return int(result.rowcount or 0)

    async def delete(self, session: AsyncSession, id_: int) -> int:
        """按 id 删除；返回 affected rows（不存在返回 0）。"""
        stmt = delete(self.model).where(
            self.model.__table__.c.id == id_  # type: ignore[attr-defined]
        )
        result = await session.execute(stmt)
        return int(result.rowcount or 0)

    async def exists(self, session: AsyncSession, id_: int) -> bool:
        return (await self.get(session, id_)) is not None
