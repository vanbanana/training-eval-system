"""审计日志模型 - Epic 23.1.

Property 14: append-only。
PG 触发器在 alembic 迁移中创建，禁止 UPDATE/DELETE。
SQLite 上无对应能力，但应用层不会发起 UPDATE/DELETE 操作。
"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    JSON,
    Boolean,
    DateTime,
    Index,
    Integer,
    String,
    Text,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base


class AuditLog(Base):
    __tablename__ = "audit_logs"
    __table_args__ = (
        Index("ix_audit_logs_occurred_user", "occurred_at", "user_id"),
        Index("ix_audit_logs_suspicious", "suspicious_flag"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    occurred_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
    user_id: Mapped[int | None] = mapped_column(Integer, nullable=True)
    username: Mapped[str] = mapped_column(
        String(64), default="", nullable=False
    )
    role: Mapped[str] = mapped_column(String(16), default="", nullable=False)
    action: Mapped[str] = mapped_column(String(64), nullable=False)
    target_type: Mapped[str] = mapped_column(
        String(32), default="", nullable=False
    )
    target_id: Mapped[str] = mapped_column(
        String(64), default="", nullable=False
    )
    target: Mapped[str] = mapped_column(
        String(128), default="", nullable=False
    )
    result: Mapped[str] = mapped_column(
        String(16), default="success", nullable=False
    )
    detail: Mapped[str] = mapped_column(Text, default="", nullable=False)
    payload: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    client_ip: Mapped[str] = mapped_column(
        String(45), default="", nullable=False
    )
    user_agent: Mapped[str] = mapped_column(
        String(255), default="", nullable=False
    )
    trace_id: Mapped[str] = mapped_column(
        String(64), default="", nullable=False
    )
    suspicious_flag: Mapped[bool] = mapped_column(
        Boolean, default=False, nullable=False
    )
    ip: Mapped[str] = mapped_column(String(45), default="", nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
