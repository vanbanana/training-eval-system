"""Profile schemas - Epic 18.5."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field


class ProfileOut(BaseModel):
    student_id: int
    radar_data: dict[str, float] | None = None
    weakness_list: list[Any] = Field(default_factory=list)
    suggestions: list[Any] = Field(default_factory=list)
    score_trend: list[Any] = Field(default_factory=list)
    source_evaluation_count: int = 0
    computed_at: datetime | None = None
    insufficient_data: bool = False
