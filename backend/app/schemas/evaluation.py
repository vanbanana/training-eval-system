"""Evaluation schemas - Epic 16.9."""

from __future__ import annotations

from datetime import datetime
from typing import Literal

from pydantic import BaseModel, Field


class DimensionScoreOut(BaseModel):
    dimension_id: int
    ai_score: float | None = None
    teacher_score: float | None = None
    rationale: str = ""


class EvaluationOut(BaseModel):
    id: int
    task_id: int
    student_id: int
    upload_id: int
    status: str
    total_score: float | None = None
    teacher_comment: str = ""
    created_at: datetime
    scores: list[DimensionScoreOut] = Field(default_factory=list)


class DimensionScoreUpdate(BaseModel):
    subj_score: float = Field(..., ge=0, le=100)
    comment: str = Field(default="", max_length=500)


class BulkActionRequest(BaseModel):
    evaluation_ids: list[int]
    action: Literal["confirm", "reject"]
    reason: str = ""


class EvaluationHistoryOut(BaseModel):
    id: int
    evaluation_id: int
    operator_id: int | None = None
    action: str
    before_value: dict | None = None
    after_value: dict | None = None
    changed_at: datetime
