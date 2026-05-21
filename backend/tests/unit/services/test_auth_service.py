"""Task 3.3 验收：AuthService 登录核心."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.config import get_settings
from app.core.database import Base
from app.core.exceptions import AccountLockedError, InvalidCredentialsError
from app.services.auth_service import LOCK_MINUTES, MAX_FAILED, login
from tests.factories.user_factory import UserFactory


pytestmark = pytest.mark.unit


@pytest.fixture(autouse=True)
def _set_jwt_secret(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


class TestLoginHappyPath:
    async def test_login_with_valid_credentials_returns_tokens(
        self, session: AsyncSession
    ) -> None:
        """Given 用户存在且密码正确；When login；Then 返回 access+refresh+user."""
        user = await UserFactory.create_async(
            session, username="alice", password="Correct123!"
        )
        await session.commit()

        resp = await login(session, username="alice", password="Correct123!")
        assert resp.access_token
        assert resp.refresh_token
        assert resp.user.username == "alice"
        # failed_login_count 重置
        await session.refresh(user)
        assert user.failed_login_count == 0
        assert user.last_login_at is not None


class TestLoginFailures:
    async def test_unknown_user_raises_invalid_credentials(
        self, session: AsyncSession
    ) -> None:
        with pytest.raises(InvalidCredentialsError) as exc:
            await login(session, username="ghost", password="any")
        assert exc.value.error_code == "INVALID_CREDENTIALS"

    async def test_wrong_password_increments_counter(self, session: AsyncSession) -> None:
        user = await UserFactory.create_async(
            session, username="bob", password="Correct123!"
        )
        await session.commit()

        with pytest.raises(InvalidCredentialsError):
            await login(session, username="bob", password="Wrong!")

        await session.refresh(user)
        assert user.failed_login_count == 1
        assert user.locked_until is None

    async def test_locks_account_after_5_failures(self, session: AsyncSession) -> None:
        """Given 已失败 4 次；When 第 5 次失败；Then 锁定 15 分钟."""
        user = await UserFactory.create_async(
            session, username="carl", password="Correct123!"
        )
        user.failed_login_count = MAX_FAILED - 1
        await session.commit()

        with pytest.raises(InvalidCredentialsError):
            await login(session, username="carl", password="bad")

        await session.refresh(user)
        assert user.failed_login_count == MAX_FAILED
        assert user.locked_until is not None
        # SQLite 可能存储为 naive datetime；统一为 UTC aware 比较
        locked_until = user.locked_until
        if locked_until.tzinfo is None:
            locked_until = locked_until.replace(tzinfo=UTC)
        delta = locked_until - datetime.now(UTC)
        assert timedelta(minutes=LOCK_MINUTES - 1) < delta <= timedelta(minutes=LOCK_MINUTES)

    async def test_locked_account_rejected_immediately(
        self, session: AsyncSession
    ) -> None:
        user = await UserFactory.create_async(session, username="locked", password="ok")
        user.locked_until = datetime.now(UTC) + timedelta(minutes=10)
        await session.commit()

        with pytest.raises(AccountLockedError) as exc:
            await login(session, username="locked", password="ok")
        assert exc.value.error_code == "ACCOUNT_LOCKED"

    async def test_disabled_account_rejected(self, session: AsyncSession) -> None:
        await UserFactory.create_async(
            session, username="disabled", password="ok", is_active=False
        )
        await session.commit()

        with pytest.raises(InvalidCredentialsError):
            await login(session, username="disabled", password="ok")


class TestUnlockAfterTimeout:
    async def test_locked_until_in_past_does_not_block(self, session: AsyncSession) -> None:
        """Given locked_until 已过期；When 用正确密码登录；Then 成功并清空锁."""
        user = await UserFactory.create_async(
            session, username="exp", password="Correct123!"
        )
        user.locked_until = datetime.now(UTC) - timedelta(minutes=1)
        user.failed_login_count = MAX_FAILED
        await session.commit()

        resp = await login(session, username="exp", password="Correct123!")
        assert resp.access_token

        await session.refresh(user)
        assert user.failed_login_count == 0
        assert user.locked_until is None
