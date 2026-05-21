"""实训任务、维度、任务-班级关联."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    CheckConstraint,
    Column,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    String,
    Table,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.core.database import Base

# 多对多关联：一个任务可关联多个班级
task_classes = Table(
    "task_classes",
    Base.metadata,
    Column(
        "task_id",
        ForeignKey("training_tasks.id", ondelete="CASCADE"),
        primary_key=True,
    ),
    Column(
        "class_id",
        ForeignKey("classes.id", ondelete="CASCADE"),
        primary_key=True,
    ),
    UniqueConstraint("task_id", "class_id", name="uq_task_classes_task_class"),
)


class TrainingTask(Base):
    __tablename__ = "training_tasks"
    __table_args__ = (
        CheckConstraint(
            "status IN ('draft', 'published', 'closed')", name="status_valid"
        ),
        Index("ix_training_tasks_teacher_status", "teacher_id", "status"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(100), nullable=False)
    description: Mapped[str] = mapped_column(Text, default="", nullable=False)
    requirements: Mapped[str] = mapped_column(Text, default="", nullable=False)
    evaluation_criteria: Mapped[str] = mapped_column(Text, default="", nullable=False)
    teacher_id: Mapped[int] = mapped_column(ForeignKey("users.id"), nullable=False)
    course_id: Mapped[int] = mapped_column(ForeignKey("courses.id"), nullable=False)
    status: Mapped[str] = mapped_column(
        String(16), default="draft", nullable=False
    )  # draft/published/closed
    deadline: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now()
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), onupdate=func.now()
    )

    dimensions: Mapped[list[Dimension]] = relationship(
        back_populates="task",
        lazy="selectin",
        cascade="all, delete-orphan",
        order_by="Dimension.order_index",
    )

    # 多对多：关联的班级
    classes = relationship(
        "Class",
        secondary=task_classes,
        lazy="selectin",
    )


class Dimension(Base):
    __tablename__ = "dimensions"
    __table_args__ = (
        CheckConstraint("weight >= 1 AND weight <= 100", name="weight_range"),
        Index("ix_dimensions_task_id", "task_id"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    task_id: Mapped[int] = mapped_column(
        ForeignKey("training_tasks.id", ondelete="CASCADE"), nullable=False
    )
    name: Mapped[str] = mapped_column(String(64), nullable=False)
    description: Mapped[str] = mapped_column(String(255), default="", nullable=False)
    weight: Mapped[int] = mapped_column(Integer, nullable=False)
    order_index: Mapped[int] = mapped_column(Integer, default=0, nullable=False)

    task: Mapped[TrainingTask] = relationship(back_populates="dimensions")
