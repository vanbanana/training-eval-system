"""ImportJob / ImportRecord 模型 - Epic 25.1."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    DateTime,
    ForeignKey,
    Index,
    Integer,
    String,
    Text,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class ImportJob(Base):
    __tablename__ = "import_jobs"
    __table_args__ = (
        Index("ix_import_jobs_operator_created", "operator_id", "created_at"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    operator_id: Mapped[int] = mapped_column(
        ForeignKey("users.id"), nullable=False
    )
    job_type: Mapped[str] = mapped_column(String(32), nullable=False)
    status: Mapped[str] = mapped_column(
        String(16), default="pending", nullable=False
    )  # pending | processing | done | failed
    total_count: Mapped[int] = mapped_column(Integer, default=0, nullable=False)
    success_count: Mapped[int] = mapped_column(
        Integer, default=0, nullable=False
    )
    failed_count: Mapped[int] = mapped_column(
        Integer, default=0, nullable=False
    )
    failed_file_path: Mapped[str | None] = mapped_column(
        String(255), nullable=True
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
    completed_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )

    records: Mapped[list[ImportRecord]] = relationship(
        back_populates="job", lazy="selectin", cascade="all, delete-orphan"
    )


class ImportRecord(Base):
    __tablename__ = "import_records"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    job_id: Mapped[int] = mapped_column(
        ForeignKey("import_jobs.id", ondelete="CASCADE"), nullable=False
    )
    row_number: Mapped[int] = mapped_column(Integer, nullable=False)
    status: Mapped[str] = mapped_column(String(16), nullable=False)
    error_message: Mapped[str] = mapped_column(
        Text, default="", nullable=False
    )

    job: Mapped[ImportJob] = relationship(back_populates="records")
