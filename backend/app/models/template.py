"""评价模板模型."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    CheckConstraint,
    DateTime,
    ForeignKey,
    Integer,
    String,
    Text,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base


class EvalTemplate(Base):
    __tablename__ = "eval_templates"
    __table_args__ = (
        CheckConstraint(
            "visibility IN ('private', 'team', 'system')",
            name="visibility_valid",
        ),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(100), nullable=False)
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    # private: 仅 owner 可见; team: 同 course 教师可见; system: 全员可见（预置）
    visibility: Mapped[str] = mapped_column(
        String(16), default="private", nullable=False
    )
    owner_id: Mapped[int | None] = mapped_column(
        ForeignKey("users.id", ondelete="SET NULL"), nullable=True
    )
    course_id: Mapped[int | None] = mapped_column(
        ForeignKey("courses.id", ondelete="SET NULL"),
        nullable=True,
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), onupdate=func.now()
    )

    items: Mapped[list[TemplateDimension]] = relationship(
        back_populates="template",
        lazy="selectin",
        cascade="all, delete-orphan",
        order_by="TemplateDimension.order_index",
    )


class TemplateDimension(Base):
    __tablename__ = "template_dimensions"
    __table_args__ = (
        CheckConstraint(
            "weight >= 1 AND weight <= 100", name="weight_range"
        ),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    template_id: Mapped[int] = mapped_column(
        ForeignKey("eval_templates.id", ondelete="CASCADE"), nullable=False
    )
    name: Mapped[str] = mapped_column(String(64), nullable=False)
    description: Mapped[str] = mapped_column(String(255), default="", nullable=False)
    weight: Mapped[int] = mapped_column(Integer, nullable=False)
    order_index: Mapped[int] = mapped_column(Integer, default=0, nullable=False)

    template: Mapped[EvalTemplate] = relationship(back_populates="items")
