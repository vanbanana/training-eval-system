"""评价模型."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    JSON,
    DateTime,
    Float,
    ForeignKey,
    Integer,
    String,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class Evaluation(Base):
    __tablename__ = "evaluations"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    task_id: Mapped[int] = mapped_column(ForeignKey("training_tasks.id"), nullable=False)
    student_id: Mapped[int] = mapped_column(ForeignKey("users.id"), nullable=False)
    upload_id: Mapped[int] = mapped_column(ForeignKey("uploads.id"), nullable=False)
    status: Mapped[str] = mapped_column(String(16), default="pending", nullable=False)  # pending/scored/confirmed/rejected
    total_score: Mapped[float | None] = mapped_column(Float, nullable=True)
    teacher_comment: Mapped[str] = mapped_column(Text, default="", nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())

    scores: Mapped[list[DimensionScore]] = relationship(back_populates="evaluation", lazy="selectin", cascade="all, delete-orphan")
    history: Mapped[list[EvaluationHistory]] = relationship(
        back_populates="evaluation", lazy="selectin", cascade="all, delete-orphan"
    )


class DimensionScore(Base):
    __tablename__ = "dimension_scores"
    __table_args__ = (
        UniqueConstraint(
            "evaluation_id", "dimension_id", name="uq_dimension_scores_eval_dim"
        ),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    evaluation_id: Mapped[int] = mapped_column(ForeignKey("evaluations.id", ondelete="CASCADE"), nullable=False)
    dimension_id: Mapped[int] = mapped_column(ForeignKey("dimensions.id"), nullable=False)
    ai_score: Mapped[float | None] = mapped_column(Float, nullable=True)
    teacher_score: Mapped[float | None] = mapped_column(Float, nullable=True)
    rationale: Mapped[str] = mapped_column(Text, default="", nullable=False)

    evaluation: Mapped[Evaluation] = relationship(back_populates="scores")


class EvaluationHistory(Base):
    """评价审计：每次修改追加一行（Property 18）."""

    __tablename__ = "evaluation_histories"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    evaluation_id: Mapped[int] = mapped_column(
        ForeignKey("evaluations.id", ondelete="CASCADE"), nullable=False
    )
    operator_id: Mapped[int | None] = mapped_column(
        ForeignKey("users.id"), nullable=True
    )
    action: Mapped[str] = mapped_column(String(32), nullable=False)
    before_value: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    after_value: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    changed_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )

    evaluation: Mapped[Evaluation] = relationship(back_populates="history")
