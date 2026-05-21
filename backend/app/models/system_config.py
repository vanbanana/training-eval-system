"""system_config - L3 业务级运行时配置.

存储可热加载的业务参数（如评分客观/主观比例、相似度阈值等）。
区别于 L2（环境变量 / Settings）的部署级配置：L3 通过管理界面修改，立即生效。
"""

from __future__ import annotations

from datetime import datetime
from typing import Any

from sqlalchemy import JSON, DateTime, ForeignKey, Integer, String, Text, UniqueConstraint, func
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base


class SystemConfig(Base):
    __tablename__ = "system_config"
    __table_args__ = (
        UniqueConstraint("key", name="uq_system_config_key"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    key: Mapped[str] = mapped_column(String(100), nullable=False)
    # value 用 JSON 存储任意结构（数字/字符串/数组/对象）
    value: Mapped[Any] = mapped_column(JSON, nullable=False)
    category: Mapped[str] = mapped_column(String(50), default="general", nullable=False)
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    updated_by: Mapped[int | None] = mapped_column(
        ForeignKey("users.id", ondelete="SET NULL"), nullable=True
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        onupdate=func.now(),
    )
