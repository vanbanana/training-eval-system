"""实训任务 schemas."""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel


class DimensionOut(BaseModel):
    id: int
    name: str
    weight: int
    order_index: int


class TaskOut(BaseModel):
    id: int
    name: str
    description: str
    requirements: str
    evaluation_criteria: str = ""
    status: str
    deadline: datetime | None
    course_id: int
    teacher_id: int
    dimensions: list[DimensionOut] = []
    created_at: datetime
