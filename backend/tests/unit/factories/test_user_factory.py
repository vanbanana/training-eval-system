"""Task 3.9 验收：UserFactory."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.security import verify_password
from app.models.user import User
from tests.factories.user_factory import (
    AdminFactory,
    TeacherFactory,
    UserFactory,
)


pytestmark = pytest.mark.unit


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestUserFactory:
    async def test_create_default_student(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session)
        assert u.id is not None
        assert u.role == "student"
        assert u.is_active is True
        # 默认密码可被 verify_password 校验
        assert verify_password("Pa$$w0rd2024", u.password_hash)

    async def test_create_with_overrides(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(
            session,
            username="custom",
            display_name="Custom",
            password="Override123!",
            is_active=False,
        )
        assert u.username == "custom"
        assert u.display_name == "Custom"
        assert u.is_active is False
        assert verify_password("Override123!", u.password_hash)

    async def test_default_username_unique(self, session: AsyncSession) -> None:
        users = [await UserFactory.create_async(session) for _ in range(5)]
        usernames = [u.username for u in users]
        assert len(set(usernames)) == 5  # 全部唯一

    async def test_chinese_display_name(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session)
        # Faker zh_CN 生成中文名，长度 ≥ 2
        assert len(u.display_name) >= 2


class TestRoleFactories:
    async def test_teacher_factory(self, session: AsyncSession) -> None:
        u = await TeacherFactory.create_async(session)
        assert u.role == "teacher"

    async def test_admin_factory(self, session: AsyncSession) -> None:
        u = await AdminFactory.create_async(session)
        assert u.role == "admin"

    async def test_role_can_be_overridden(self, session: AsyncSession) -> None:
        u = await UserFactory.create_async(session, role="admin")
        assert u.role == "admin"
