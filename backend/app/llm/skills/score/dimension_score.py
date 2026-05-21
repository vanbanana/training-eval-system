"""score.dimension Skill - 单个维度的客观打分."""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel, Field, field_validator

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class DimensionInfo(BaseModel):
    name: str
    description: str = ""


class DimensionScoreInput(BaseModel):
    task_requirements: str
    dimension: DimensionInfo
    parse_summary: str = ""
    verify_report: str = ""


class DimensionScoreOutput(BaseModel):
    score: int = Field(..., ge=0, le=100)
    rationale: str

    @field_validator("rationale")
    @classmethod
    def rationale_min_length(cls, v: str) -> str:
        if len(v) < 50:
            raise ValueError("rationale 必须 ≥ 50 字符")
        return v


class DimensionScoreSkill(Skill[DimensionScoreInput, DimensionScoreOutput]):
    name: ClassVar[str] = "score.dimension"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "score"
    temperature: ClassVar[float] = 0.0
    input_schema: ClassVar[type[BaseModel]] = DimensionScoreInput
    output_schema: ClassVar[type[BaseModel]] = DimensionScoreOutput

    def render_prompt(self, input_: DimensionScoreInput) -> list[LLMMessage]:
        sys = (
            "你是严格的实训评分助手。"
            "针对单个维度给出 0-100 分整数 + ≥50 字符可执行的扣分理由。"
            "只输出严格 JSON。"
        )
        user = (
            f"# 任务要求\n{input_.task_requirements}\n\n"
            f"# 维度\n名称：{input_.dimension.name}\n"
            f"说明：{input_.dimension.description}\n\n"
            f"# 提交摘要\n{input_.parse_summary}\n\n"
            f"# 核查报告\n{input_.verify_report}\n\n"
            "# 输出 JSON\n"
            '{"score": 85, "rationale": "至少 50 字符的具体扣分理由..."}'
        )
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]
