"""学生上传 + 解析结果 + 核查结果."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    JSON,
    BigInteger,
    CheckConstraint,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    Numeric,
    String,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class Upload(Base):
    __tablename__ = "uploads"
    __table_args__ = (
        CheckConstraint(
            "parse_status IN ('pending', 'parsing', 'parsed', 'failed')",
            name="parse_status_valid",
        ),
        Index("ix_uploads_task_student", "task_id", "student_id"),
        Index("ix_uploads_status", "parse_status"),
        Index("ix_uploads_sha256", "student_id", "sha256"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    task_id: Mapped[int] = mapped_column(
        ForeignKey("training_tasks.id"), nullable=False
    )
    student_id: Mapped[int] = mapped_column(
        ForeignKey("users.id"), nullable=False
    )
    filename: Mapped[str] = mapped_column(String(255), nullable=False)
    file_type: Mapped[str] = mapped_column(String(16), nullable=False)
    file_size: Mapped[int] = mapped_column(Integer, nullable=False)
    storage_path: Mapped[str] = mapped_column(String(512), nullable=False)
    sha256: Mapped[str] = mapped_column(String(64), default="", nullable=False)
    parse_status: Mapped[str] = mapped_column(
        String(16), default="pending", nullable=False
    )
    version: Mapped[int] = mapped_column(Integer, default=1, nullable=False)
    is_deleted: Mapped[bool] = mapped_column(Integer, default=0, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), onupdate=func.now()
    )

    parse_result: Mapped[ParseResult | None] = relationship(
        back_populates="upload",
        lazy="selectin",
        cascade="all, delete-orphan",
        uselist=False,
    )
    verify_result: Mapped[VerifyResult | None] = relationship(
        back_populates="upload",
        lazy="selectin",
        cascade="all, delete-orphan",
        uselist=False,
    )


class ParseResult(Base):
    __tablename__ = "parse_results"
    __table_args__ = (
        UniqueConstraint("upload_id", name="uq_parse_results_upload_id"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    upload_id: Mapped[int] = mapped_column(
        ForeignKey("uploads.id", ondelete="CASCADE"), nullable=False
    )
    structured_content: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    raw_text: Mapped[str] = mapped_column(Text, default="", nullable=False)
    simhash: Mapped[int | None] = mapped_column(BigInteger, nullable=True)
    # 实际生产用 pgvector；dev sqlite 用 JSON 数组兼容
    embedding: Mapped[list[float] | None] = mapped_column(JSON, nullable=True)
    error_message: Mapped[str] = mapped_column(Text, default="", nullable=False)
    parsed_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )

    upload: Mapped[Upload] = relationship(back_populates="parse_result")


class VerifyResult(Base):
    __tablename__ = "verify_results"
    __table_args__ = (
        UniqueConstraint("upload_id", name="uq_verify_results_upload_id"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    upload_id: Mapped[int] = mapped_column(
        ForeignKey("uploads.id", ondelete="CASCADE"), nullable=False
    )
    match_rate: Mapped[float | None] = mapped_column(Numeric(5, 2), nullable=True)
    checkpoints: Mapped[list | None] = mapped_column(JSON, nullable=True)
    missing_items: Mapped[list | None] = mapped_column(JSON, nullable=True)
    logic_issues: Mapped[list | None] = mapped_column(JSON, nullable=True)
    overall_confidence: Mapped[int | None] = mapped_column(Integer, nullable=True)
    verified_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )

    upload: Mapped[Upload] = relationship(back_populates="verify_result")
