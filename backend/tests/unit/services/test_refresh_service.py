"""Task 3.4 验收：RefreshTokenService."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import timedelta

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.config import get_settings
from app.core.database import Base
from app.core.exceptions import AuthenticationError
from app.core.security import create_access_token, create_refresh_token
from app.services.auth_service import refresh_tokens
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


class TestRefreshHappyPath:
    async def test_valid_refresh_returns_new_tokens(self, session: AsyncSession) -> None:
        user = await UserFactory.create_async(session, role="teacher")
        await session.commit()

        refresh = create_refresh_token(user_id=user.id)
        resp = await refresh_tokens(session, refresh_token=refresh)
        assert resp.access_token
        assert resp.refresh_token
        assert resp.user.username == user.username


class TestRefreshFailures:
    async def test_invalid_jwt_raises(self, session: AsyncSession) -> None:
        with pytest.raises(AuthenticationError):
            await refresh_tokens(session, refresh_token="not.a.token")

    async def test_access_token_used_as_refresh_rejected(
        self, session: AsyncSession
    ) -> None:
        """Given access token；When 当成 refresh 用；Then 报 INVALID_TOKEN_TYPE。"""
        user = await UserFactory.create_async(session)
        await session.commit()

        access = create_access_token(user_id=user.id, role=user.role)
        with pytest.raises(AuthenticationError) as exc:
            await refresh_tokens(session, refresh_token=access)
        assert "type" in str(exc.value).lower() or "TOKEN_TYPE" in (exc.value.error_code or "")

    async def test_user_disabled_after_token_issued(self, session: AsyncSession) -> None:
        user = await UserFactory.create_async(session, is_active=False)
        await session.commit()

        refresh = create_refresh_token(user_id=user.id)
        with pytest.raises(AuthenticationError) as exc:
            await refresh_tokens(session, refresh_token=refresh)
        # 错误码应为 USER_DISABLED 或包含相关信息
        assert "DISABLED" in (exc.value.error_code or "") or "禁用" in str(exc.value)

    async def test_user_deleted_returns_user_not_found(self, session: AsyncSession) -> None:
        # 签 token 但用户不存在
        refresh = create_refresh_token(user_id=99999)
        with pytest.raises(AuthenticationError):
            await refresh_tokens(session, refresh_token=refresh)


class TestExpiredRefresh:
    async def test_expired_refresh_token_rejected(self, session: AsyncSession) -> None:
        from app.core.security import jwt as jose_jwt
        from datetime import UTC, datetime
        from app.core.config import get_settings as gs

        s = gs()
        # 手动构造已过期的 refresh
        expired_payload = {
            "sub": "1",
            "iat": int((datetime.now(UTC) - timedelta(days=10)).timestamp()),
            "exp": int((datetime.now(UTC) - timedelta(days=1)).timestamp()),
            "type": "refresh",
        }
        expired = jose_jwt.encode(expired_payload, s.jwt_secret, algorithm=s.jwt_algorithm)

        with pytest.raises(AuthenticationError):
            await refresh_tokens(session, refresh_token=expired)
