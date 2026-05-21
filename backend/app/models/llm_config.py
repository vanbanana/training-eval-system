"""LLM 配置模型."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, Integer, String, Text, func
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base


class LlmConfig(Base):
    __tablename__ = "llm_configs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    provider: Mapped[str] = mapped_column(String(32), nullable=False)  # deepseek/tongyi/zhipu/moonshot
    base_url: Mapped[str] = mapped_column(String(255), nullable=False)
    api_key_encrypted: Mapped[str] = mapped_column(Text, nullable=False)  # AES-256 encrypted
    chat_model: Mapped[str] = mapped_column(String(64), nullable=False)
    embed_model: Mapped[str] = mapped_column(String(64), default="", nullable=False)
    is_active: Mapped[bool] = mapped_column(default=True, nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())
