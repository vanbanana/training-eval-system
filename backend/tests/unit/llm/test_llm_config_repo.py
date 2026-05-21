"""Task 10.5 验收：LlmConfig 加密存储."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.config import get_settings
from app.core.database import Base
from app.repositories.llm_config_repo import LLMConfigRepository


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


class TestUpsertEncryption:
    async def test_writes_encrypted_not_plaintext(
        self, session: AsyncSession
    ) -> None:
        repo = LLMConfigRepository()
        cfg = await repo.upsert(
            session,
            provider="deepseek",
            base_url="https://api.deepseek.com/v1",
            api_key_plain="sk-secret-1234567890",
            chat_model="deepseek-chat",
        )
        await session.commit()

        # DB 中 api_key_encrypted 不能是原文
        assert cfg.api_key_encrypted != "sk-secret-1234567890"
        # 解密后 == 原文
        assert (
            LLMConfigRepository.decrypt_api_key(cfg)
            == "sk-secret-1234567890"
        )

    async def test_only_one_active(self, session: AsyncSession) -> None:
        repo = LLMConfigRepository()
        await repo.upsert(
            session,
            provider="a",
            base_url="https://a",
            api_key_plain="k1",
            chat_model="m1",
        )
        await session.commit()
        await repo.upsert(
            session,
            provider="b",
            base_url="https://b",
            api_key_plain="k2",
            chat_model="m2",
        )
        await session.commit()

        active = await repo.get_active(session)
        assert active is not None
        assert active.provider == "b"

        from sqlalchemy import select
        from app.models.llm_config import LlmConfig

        all_cfgs = (await session.execute(select(LlmConfig))).scalars().all()
        actives = [c for c in all_cfgs if c.is_active]
        assert len(actives) == 1


class TestMaskApiKey:
    def test_mask_format(self) -> None:
        masked = LLMConfigRepository.mask_api_key("AAAAB3NzaC1yc2EAAAAD")
        assert masked.startswith("sk-***")
        assert "AAAAB3" not in masked

    def test_short_string_fully_masked(self) -> None:
        assert LLMConfigRepository.mask_api_key("a") == "***"
