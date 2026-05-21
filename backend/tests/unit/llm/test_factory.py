"""Task 10.6 验收：LLMFactory 运行时切换."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.config import get_settings
from app.core.database import Base
from app.core.exceptions import LLMUnavailableError
from app.llm.factory import LLMFactory
from app.repositories.llm_config_repo import LLMConfigRepository
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


@pytest.fixture(autouse=True)
def _set_jwt(monkeypatch: pytest.MonkeyPatch) -> None:
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


class TestFactory:
    async def test_no_config_raises(self, session: AsyncSession) -> None:
        factory = LLMFactory()
        with pytest.raises(LLMUnavailableError):
            await factory.current(session)

    async def test_test_provider_override(self, session: AsyncSession) -> None:
        factory = LLMFactory()
        fake = FakeLLM()
        factory.set_test_provider(fake)

        provider = await factory.current(session)
        assert provider is fake

    async def test_provider_cached_until_config_changes(
        self, session: AsyncSession
    ) -> None:
        repo = LLMConfigRepository()
        await repo.upsert(
            session,
            provider="x",
            base_url="https://api/v1",
            api_key_plain="k",
            chat_model="m",
        )
        await session.commit()

        factory = LLMFactory()
        p1 = await factory.current(session)
        p2 = await factory.current(session)
        assert p1 is p2  # 同一实例

        # 更新配置 → 重建
        await repo.upsert(
            session,
            provider="y",
            base_url="https://api2/v1",
            api_key_plain="k2",
            chat_model="m2",
        )
        await session.commit()

        p3 = await factory.current(session)
        assert p3 is not p1
