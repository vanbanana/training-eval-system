"""LlmConfig Repository - 加密 API Key 透明读写."""

from __future__ import annotations

import base64

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.config import get_settings
from app.core.crypto import decrypt, derive_master_key, encrypt
from app.models.llm_config import LlmConfig
from app.repositories.base import BaseRepository


def _master_key() -> bytes:
    return derive_master_key(get_settings().llm_key_master)


class LLMConfigRepository(BaseRepository[LlmConfig]):
    model = LlmConfig

    async def get_active(self, db: AsyncSession) -> LlmConfig | None:
        stmt = (
            select(LlmConfig)
            .where(LlmConfig.is_active.is_(True))
            .order_by(LlmConfig.id.desc())
        )
        return (await db.execute(stmt)).scalar_one_or_none()

    async def upsert(
        self,
        db: AsyncSession,
        *,
        provider: str,
        base_url: str,
        api_key_plain: str,
        chat_model: str,
        embed_model: str = "",
    ) -> LlmConfig:
        cipher = encrypt(api_key_plain, _master_key())

        # 关闭其他 active config
        for old in (await db.execute(select(LlmConfig))).scalars().all():
            old.is_active = False

        cfg = LlmConfig(
            provider=provider,
            base_url=base_url,
            api_key_encrypted=cipher,
            chat_model=chat_model,
            embed_model=embed_model,
            is_active=True,
        )
        db.add(cfg)
        await db.flush()
        await db.refresh(cfg)
        return cfg

    @staticmethod
    def decrypt_api_key(cfg: LlmConfig) -> str:
        return decrypt(cfg.api_key_encrypted, _master_key())

    @staticmethod
    def mask_api_key(plain_or_encrypted: str) -> str:
        """对 API Key 做 mask 用于响应."""
        # 不解密，直接用密文长度 + 后 4 位 base64
        if len(plain_or_encrypted) <= 8:
            return "***"
        try:
            raw = base64.b64decode(plain_or_encrypted, validate=False)
            tail = raw[-3:].hex() if len(raw) >= 3 else "?"
        except Exception:  # noqa: BLE001
            tail = plain_or_encrypted[-4:]
        return f"sk-***...***{tail}"
