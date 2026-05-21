"""Task 3.2 验收：UserRepository."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.models.user import User
from app.repositories.user_repo import UserRepository


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


@pytest.fixture()
async def repo(session: AsyncSession) -> UserRepository:
    return UserRepository()


@pytest.fixture()
async def seeded_users(session: AsyncSession) -> list[User]:
    users = [
        User(username="alice", display_name="Alice", password_hash="x", role="student"),
        User(username="Bob", display_name="Bob", password_hash="x", role="teacher"),
        User(username="charlie", display_name="C", password_hash="x", role="admin"),
        User(
            username="dora",
            display_name="D",
            password_hash="x",
            role="student",
            is_active=False,
        ),
    ]
    for u in users:
        session.add(u)
    await session.commit()
    for u in users:
        await session.refresh(u)
    return users


class TestGetByUsername:
    async def test_find_existing(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        user = await repo.get_by_username(session, "alice")
        assert user is not None
        assert user.display_name == "Alice"

    async def test_case_insensitive_lookup(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        """Given DB 中 username='Bob'；When 查 'bob' / 'BOB'；Then 都能命中。"""
        u1 = await repo.get_by_username(session, "bob")
        u2 = await repo.get_by_username(session, "BOB")
        assert u1 is not None and u1.username == "Bob"
        assert u2 is not None and u2.username == "Bob"

    async def test_not_found_returns_none(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        assert await repo.get_by_username(session, "nobody") is None


class TestListByRole:
    async def test_filter_students(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        students = await repo.list_by_role(session, "student")
        assert len(students) == 2
        assert all(u.role == "student" for u in students)

    async def test_filter_teachers(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        teachers = await repo.list_by_role(session, "teacher")
        assert len(teachers) == 1
        assert teachers[0].username == "Bob"


class TestListActive:
    async def test_excludes_disabled_users(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        active = await repo.list_active(session)
        # dora 被禁用
        usernames = [u.username for u in active]
        assert "dora" not in usernames
        assert len(active) == 3


class TestInheritedCRUD:
    async def test_get_returns_user(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        u = await repo.get(session, seeded_users[0].id)
        assert u is not None
        assert u.username == "alice"

    async def test_count(
        self, session: AsyncSession, repo: UserRepository, seeded_users: list[User]
    ) -> None:
        assert await repo.count(session) == 4
