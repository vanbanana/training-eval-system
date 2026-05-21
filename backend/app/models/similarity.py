"""SimilarityRecord 模型 - Epic 17.2."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    CheckConstraint,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    Numeric,
    String,
    UniqueConstraint,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base


class SimilarityRecord(Base):
    __tablename__ = "similarity_records"
    __table_args__ = (
        # SQLite 上无法用 LEAST/GREATEST 表达式索引；
        # 业务层在写入前保证 upload_a < upload_b 以保留唯一性
        UniqueConstraint(
            "task_id", "upload_a_id", "upload_b_id", name="uq_sim_task_pair"
        ),
        CheckConstraint(
            "state IN ('suspect', 'confirmed', 'ignored')", name="state_valid"
        ),
        CheckConstraint(
            "upload_a_id < upload_b_id", name="upload_a_lt_b"
        ),
        Index("ix_similarity_task_state", "task_id", "state"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    task_id: Mapped[int] = mapped_column(
        ForeignKey("training_tasks.id"), nullable=False
    )
    upload_a_id: Mapped[int] = mapped_column(
        ForeignKey("uploads.id"), nullable=False
    )
    upload_b_id: Mapped[int] = mapped_column(
        ForeignKey("uploads.id"), nullable=False
    )
    hamming_distance: Mapped[int] = mapped_column(Integer, nullable=False)
    cosine_similarity: Mapped[float | None] = mapped_column(
        Numeric(4, 3), nullable=True
    )
    state: Mapped[str] = mapped_column(
        String(16), default="suspect", nullable=False
    )
    reviewed_by: Mapped[int | None] = mapped_column(
        ForeignKey("users.id"), nullable=True
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
    decided_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
