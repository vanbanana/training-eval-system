"""Task 3.1 验收：User 模型与表."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import inspect, text
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.security import hash_password
from app.models.user import User


pytestmark = pytest.mark.integration


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestUserSchema:
    async def test_can_create_with_required_fields(self, session: AsyncSession) -> None:
        u = User(
            username="alice",
            display_name="Alice",
            password_hash=hash_password("Pwd12345!"),
            role="student",
        )
        session.add(u)
        await session.commit()
        assert u.id is not None
        assert u.is_active is True
        assert u.failed_login_count == 0
        assert u.locked_until is None
        assert u.created_at is not None

    async def test_username_uniqueness(self, session: AsyncSession) -> None:
        u1 = User(username="bob", display_name="Bob", password_hash="x", role="student")
        u2 = User(username="bob", display_name="Bob2", password_hash="x", role="teacher")
        session.add(u1)
        await session.commit()
        session.add(u2)
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestRoleConstraint:
    async def test_valid_roles_accepted(self, session: AsyncSession) -> None:
        for role in ("admin", "teacher", "student"):
            u = User(
                username=f"user_{role}",
                display_name=role,
                password_hash="x",
                role=role,
            )
            session.add(u)
        await session.commit()

    async def test_invalid_role_rejected(self, session: AsyncSession) -> None:
        """Given role='invalid'；When commit；Then IntegrityError（CHECK 约束）."""
        u = User(
            username="bad",
            display_name="Bad",
            password_hash="x",
            role="invalid",
        )
        session.add(u)
        with pytest.raises(IntegrityError):
            await session.commit()
        await session.rollback()


class TestIndices:
    async def test_username_indexed(self, session: AsyncSession) -> None:
        from sqlalchemy import inspect as sa_inspect

        def _get_indexes(conn: object) -> list[dict[str, object]]:
            return sa_inspect(conn).get_indexes(User.__tablename__)  # type: ignore[arg-type, return-value]

        async with session.bind.connect() as conn:  # type: ignore[union-attr]
            indexes = await conn.run_sync(_get_indexes)

        # 至少 1 个索引指向 username
        index_columns = [tuple(idx["column_names"]) for idx in indexes]
        assert any("username" in cols for cols in index_columns)
