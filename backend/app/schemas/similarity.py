"""Similarity schemas - Epic 17.6."""

from __future__ import annotations

from datetime import datetime
from typing import Literal

from pydantic import BaseModel


class SimilarityRecordOut(BaseModel):
    id: int
    task_id: int
    upload_a_id: int
    upload_b_id: int
    hamming_distance: int
    cosine_similarity: float | None = None
    state: str
    created_at: datetime
    decided_at: datetime | None = None


class SimilarityDecision(BaseModel):
    action: Literal["confirm", "ignore"]


class SegmentPair(BaseModel):
    a_start: int
    a_end: int
    b_start: int
    b_end: int
    snippet_a: str
    snippet_b: str
    ratio: float
