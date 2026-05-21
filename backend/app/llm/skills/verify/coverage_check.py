"""verify.coverage_check Skill - 检查实训成果对要求清单的覆盖度."""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel, Field, field_validator

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class Checkpoint(BaseModel):
    requirement: str
    matched: bool
    evidence: str = ""
    confidence: int = Field(..., ge=0, le=100)


class CoverageInput(BaseModel):
    task_requirements: str
    parse_summary: str = ""
    parse_key_points: list[str] = Field(default_factory=list)


class CoverageOutput(BaseModel):
    checkpoints: list[Checkpoint] = Field(default_factory=list)

    @field_validator("checkpoints")
    @classmethod
    def must_not_be_empty(cls, v: list[Checkpoint]) -> list[Checkpoint]:
        if not v:
            raise ValueError("checkpoints 不能为空")
        return v


class CoverageCheckSkill(Skill[CoverageInput, CoverageOutput]):
    name: ClassVar[str] = "verify.coverage_check"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "verify"
    temperature: ClassVar[float] = 0.0
    input_schema: ClassVar[type[BaseModel]] = CoverageInput
    output_schema: ClassVar[type[BaseModel]] = CoverageOutput

    def render_prompt(self, input_: CoverageInput) -> list[LLMMessage]:
        sys = (
            "你是严格的实训成果核查助手。"
            "对实训要求清单中的每个条目，判断学生提交是否覆盖。"
            "只能返回严格 JSON，不要任何额外文本。"
        )
        user = (
            f"# 实训要求\n{input_.task_requirements}\n\n"
            f"# 提交摘要\n{input_.parse_summary}\n\n"
            f"# 提交要点\n- "
            + "\n- ".join(input_.parse_key_points or ["(无)"])
            + "\n\n"
            "# 输出 JSON\n"
            '{"checkpoints":[{"requirement":"...","matched":true,'
            '"evidence":"...","confidence":85}]}'
        )
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]
