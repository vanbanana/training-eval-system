"""SQLAlchemy 2.0 async 引擎与 session.

约定：
- 所有 ORM 模型继承 Base
- 命名约定通过 MetaData(naming_convention) 定义，避免 Alembic 生成无名约束
- get_db_session() 是 FastAPI 依赖，每请求一个 session，自动 commit/rollback/close
"""

from __future__ import annotations

from collections.abc import AsyncIterator

from sqlalchemy import MetaData
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)
from sqlalchemy.orm import DeclarativeBase

from app.core.config import get_settings

# Alembic 友好的命名约定
NAMING_CONVENTION = {
    "ix": "ix_%(column_0_label)s",
    "uq": "uq_%(table_name)s_%(column_0_name)s",
    "ck": "ck_%(table_name)s_%(constraint_name)s",
    "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s",
    "pk": "pk_%(table_name)s",
}


class Base(DeclarativeBase):
    """所有 ORM 模型基类."""

    metadata = MetaData(naming_convention=NAMING_CONVENTION)


def _create_engine(db_url: str, echo: bool) -> AsyncEngine:
    """创建 async engine；SQLite 与 PG 用不同的 pool 参数."""
    if db_url.startswith("sqlite"):
        # SQLite async 不支持连接池 pre_ping；不传 pool_pre_ping
        return create_async_engine(db_url, echo=echo, future=True)
    return create_async_engine(
        db_url,
        echo=echo,
        pool_pre_ping=True,
        pool_size=10,
        max_overflow=10,
        pool_recycle=1800,
        future=True,
    )


_settings = get_settings()
engine: AsyncEngine = _create_engine(_settings.db_url, echo=_settings.debug)
SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)


async def get_db_session() -> AsyncIterator[AsyncSession]:
    """FastAPI 依赖：每请求一个 session.

    - 正常返回：commit
    - 异常：rollback
    - 总是 close
    """
    async with SessionLocal() as session:
        try:
            yield session
            await session.commit()
        except Exception:
            await session.rollback()
            raise


# 兼容旧导入名
get_db = get_db_session


__all__ = [
    "NAMING_CONVENTION",
    "Base",
    "SessionLocal",
    "engine",
    "get_db",
    "get_db_session",
]
