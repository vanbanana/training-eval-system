"""profile.teaching_summary Skill - Epic 19.3."""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel, Field

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class CommonWeakness(BaseModel):
    dimension_name: str
    student_ratio: float = Field(..., ge=0, le=1)
    avg_score: float


class TeachingSummaryInput(BaseModel):
    course_name: str = ""
    total_students: int = 0
    total_evaluations: int = 0
    avg_score: float = 0.0
    dimension_distributions: list[dict] = Field(default_factory=list)


class TeachingSummaryOutput(BaseModel):
    summary: str
    common_weaknesses: list[CommonWeakness] = Field(default_factory=list)
    suggestions: list[str] = Field(default_factory=list)


class TeachingSummarySkill(
    Skill[TeachingSummaryInput, TeachingSummaryOutput]
):
    name: ClassVar[str] = "profile.teaching_summary"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "profile"
    temperature: ClassVar[float] = 0.2
    input_schema: ClassVar[type[BaseModel]] = TeachingSummaryInput
    output_schema: ClassVar[type[BaseModel]] = TeachingSummaryOutput

    def render_prompt(self, input_: TeachingSummaryInput) -> list[LLMMessage]:
        sys = (
            "你是教学质量分析助手。"
            "基于课程级聚合数据，给出整体小结+共性薄弱点+教学建议。"
            "common_weaknesses 仅纳入低分（<60）学生比例 > 30% 的维度。"
            "只输出严格 JSON。"
        )
        rows = "\n".join(
            f"- {d.get('name', '?')}: 均分 {d.get('avg', 0)}, "
            f"<60 比例 {d.get('low_ratio', 0):.2f}"
            for d in input_.dimension_distributions
        )
        user = (
            f"# 课程\n{input_.course_name}\n"
            f"# 总数\n学生：{input_.total_students}, "
            f"评价：{input_.total_evaluations}, 均分：{input_.avg_score}\n"
            f"# 维度分布\n{rows}\n\n"
            "# 输出 JSON\n"
            '{"summary":"...","common_weaknesses":[{"dimension_name":"...",'
            '"student_ratio":0.4,"avg_score":55.0}],'
            '"suggestions":["..."]}'
        )
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]


__all__ = [
    "CommonWeakness",
    "TeachingSummaryInput",
    "TeachingSummaryOutput",
    "TeachingSummarySkill",
]
