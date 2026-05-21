"""StudentProfile 模型 - Epic 18.1."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    JSON,
    DateTime,
    ForeignKey,
    Integer,
    UniqueConstraint,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column

from app.core.database import Base


class StudentProfile(Base):
    __tablename__ = "student_profiles"
    __table_args__ = (
        UniqueConstraint("student_id", name="uq_student_profiles_student_id"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    student_id: Mapped[int] = mapped_column(
        ForeignKey("users.id"), nullable=False
    )
    radar_data: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    weakness_list: Mapped[list | None] = mapped_column(JSON, nullable=True)
    suggestions: Mapped[list | None] = mapped_column(JSON, nullable=True)
    score_trend: Mapped[list | None] = mapped_column(JSON, nullable=True)
    source_evaluation_count: Mapped[int] = mapped_column(
        Integer, default=0, nullable=False
    )
    computed_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
