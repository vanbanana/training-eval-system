"""profile.weakness_analyze Skill."""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel, Field

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class DimensionStat(BaseModel):
    name: str
    avg_score: float
    min_score: float
    count: int


class Weakness(BaseModel):
    dimension_name: str
    mastery_score: float = Field(..., ge=0, le=100)
    related_evaluation_ids: list[int] = Field(default_factory=list)
    reason: str


class WeaknessInput(BaseModel):
    dimension_stats: list[DimensionStat]


class WeaknessOutput(BaseModel):
    weaknesses: list[Weakness] = Field(default_factory=list)


class WeaknessAnalyzeSkill(Skill[WeaknessInput, WeaknessOutput]):
    name: ClassVar[str] = "profile.weakness_analyze"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "profile"
    temperature: ClassVar[float] = 0.0
    input_schema: ClassVar[type[BaseModel]] = WeaknessInput
    output_schema: ClassVar[type[BaseModel]] = WeaknessOutput

    def render_prompt(self, input_: WeaknessInput) -> list[LLMMessage]:
        sys = (
            "你是学习诊断助手。基于学生历次维度评分，识别薄弱维度。"
            "只输出严格 JSON。"
        )
        rows = "\n".join(
            f"- {d.name}: 平均 {d.avg_score:.1f}, 最低 {d.min_score:.1f}, 次数 {d.count}"
            for d in input_.dimension_stats
        )
        user = (
            f"# 维度统计\n{rows}\n\n"
            "# 输出 JSON\n"
            '{"weaknesses":[{"dimension_name":"...",'
            '"mastery_score":65.0,"related_evaluation_ids":[],'
            '"reason":"具体原因..."}]}'
        )
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]
