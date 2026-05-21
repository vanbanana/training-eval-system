"""模板 schemas."""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field


class TemplateDimensionInput(BaseModel):
    name: str = Field(..., min_length=1, max_length=64)
    description: str = ""
    weight: int = Field(..., ge=1, le=100)


class TemplateDimensionOut(BaseModel):
    id: int
    name: str
    description: str
    weight: int
    order_index: int


class TemplateOut(BaseModel):
    id: int
    name: str
    description: str
    visibility: str
    owner_id: int | None
    course_id: int | None
    items: list[TemplateDimensionOut] = []
    created_at: datetime


class CreateTemplateRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    description: str = ""
    visibility: str = Field(default="private", pattern="^(private|team|system)$")
    course_id: int | None = None
    dimensions: list[TemplateDimensionInput] = Field(default_factory=list)


class ApplyTemplateRequest(BaseModel):
    task_id: int


class SaveFromTaskRequest(BaseModel):
    task_id: int
    name: str = Field(..., min_length=1, max_length=100)
    description: str = ""
    visibility: str = Field(default="private", pattern="^(private|team|system)$")
    course_id: int | None = None
