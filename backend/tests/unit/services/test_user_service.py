"""Task 3.7 验收：UserService."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import AuthorizationError, ConflictError, ResourceNotFoundError
from app.services.user_service import UserService
from tests.factories.user_factory import AdminFactory, TeacherFactory, UserFactory


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


@pytest.fixture()
def svc() -> UserService:
    return UserService()


class TestAuthorizationGuard:
    async def test_non_admin_rejected_on_create(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="t1")
        await session.commit()

        with pytest.raises(AuthorizationError):
            await svc.create_user(
                session,
                actor=teacher,
                data={
                    "username": "x",
                    "display_name": "X",
                    "role": "student",
                    "password": "Pwd12345!",
                },
            )

    async def test_non_admin_rejected_on_list(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        student = await UserFactory.create_async(session, username="s1")
        await session.commit()

        with pytest.raises(AuthorizationError):
            await svc.list_users(session, actor=student)


class TestCreate:
    async def test_create_user_with_valid_data(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="admin1")
        await session.commit()

        new_user = await svc.create_user(
            session,
            actor=admin,
            data={
                "username": "newcomer",
                "display_name": "New Comer",
                "role": "teacher",
                "password": "Pwd12345!",
            },
        )
        await session.commit()

        assert new_user.id is not None
        assert new_user.role == "teacher"
        assert new_user.password_hash != "Pwd12345!"

    async def test_duplicate_username_raises_conflict(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="a")
        await UserFactory.create_async(session, username="dup")
        await session.commit()

        with pytest.raises(ConflictError) as exc:
            await svc.create_user(
                session,
                actor=admin,
                data={
                    "username": "dup",
                    "display_name": "X",
                    "role": "student",
                    "password": "Pwd12345!",
                },
            )
        assert exc.value.field == "username"

    async def test_invalid_role_raises_conflict(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="a2")
        await session.commit()

        with pytest.raises(ConflictError) as exc:
            await svc.create_user(
                session,
                actor=admin,
                data={
                    "username": "x",
                    "display_name": "X",
                    "role": "godmode",
                    "password": "Pwd12345!",
                },
            )
        assert exc.value.field == "role"


class TestUpdate:
    async def test_update_display_name(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="a3")
        target = await UserFactory.create_async(session, username="t", display_name="Old")
        await session.commit()

        updated = await svc.update_user(
            session,
            actor=admin,
            user_id=target.id,
            data={"display_name": "New"},
        )
        await session.commit()

        assert updated.display_name == "New"

    async def test_update_nonexistent_user(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="a4")
        await session.commit()

        with pytest.raises(ResourceNotFoundError):
            await svc.update_user(session, actor=admin, user_id=99999, data={"display_name": "x"})

    async def test_deactivate_user(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="a5")
        target = await UserFactory.create_async(session, username="t2")
        await session.commit()

        result = await svc.deactivate_user(session, actor=admin, user_id=target.id)
        await session.commit()
        assert result.is_active is False


class TestPasswordReset:
    async def test_reset_password_changes_hash(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="a6")
        target = await UserFactory.create_async(session, username="t3", password="Old123!")
        old_hash = target.password_hash
        await session.commit()

        await svc.reset_password(
            session, actor=admin, user_id=target.id, new_password="New456!"
        )
        await session.commit()
        await session.refresh(target)
        assert target.password_hash != old_hash


class TestList:
    async def test_list_filtered_by_role(
        self, session: AsyncSession, svc: UserService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="al")
        for i in range(3):
            await UserFactory.create_async(session, username=f"s{i}")
        for i in range(2):
            await TeacherFactory.create_async(session, username=f"te{i}")
        await session.commit()

        teachers = await svc.list_users(session, actor=admin, role="teacher")
        assert len(teachers) == 2
        assert all(u.role == "teacher" for u in teachers)
