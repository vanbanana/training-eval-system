"""LLM Factory - 运行时基于 LlmConfig 选择 Provider."""

from __future__ import annotations

from typing import TYPE_CHECKING

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import LLMUnavailableError
from app.core.logging import get_logger
from app.repositories.llm_config_repo import LLMConfigRepository


if TYPE_CHECKING:
    from app.llm.base import LLMProvider


log = get_logger(__name__)


class LLMFactory:
    """从 DB 读取 active 配置构造 Provider；支持运行时热切换."""

    def __init__(self) -> None:
        self._cached: tuple[int, "LLMProvider"] | None = None  # (config_id, provider)
        self._test_provider: "LLMProvider | None" = None

    def set_test_provider(self, provider: "LLMProvider | None") -> None:
        """测试用：注入 fake provider 完全绕过 DB."""
        self._test_provider = provider
        self._cached = None

    async def current(self, db: AsyncSession) -> "LLMProvider":
        if self._test_provider is not None:
            return self._test_provider

        repo = LLMConfigRepository()
        cfg = await repo.get_active(db)
        if cfg is None:
            raise LLMUnavailableError(
                "未配置 LLM；请管理员先配置", field="llm_config"
            )

        # 缓存命中：使用同一 provider 实例
        if self._cached is not None and self._cached[0] == cfg.id:
            return self._cached[1]

        # 重建 provider
        provider = self._build_provider(cfg)
        self._cached = (cfg.id, provider)
        log.info(
            "llm.factory.provider_built",
            provider=cfg.provider,
            chat_model=cfg.chat_model,
        )
        return provider

    @staticmethod
    def _build_provider(cfg: object) -> "LLMProvider":
        """根据 DB 配置构建 OpenAI 兼容 Provider."""
        from app.llm.openai_compat import OpenAICompatProvider

        api_key = LLMConfigRepository.decrypt_api_key(cfg)  # type: ignore[arg-type]
        return OpenAICompatProvider(
            base_url=cfg.base_url,  # type: ignore[attr-defined]
            api_key=api_key,
            model=cfg.chat_model,  # type: ignore[attr-defined]
            embed_model=getattr(cfg, "embed_model", "") or "",
            timeout=60.0,
        )


# 模块级单例
llm_factory = LLMFactory()
