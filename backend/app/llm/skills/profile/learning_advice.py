"""profile.learning_advice Skill."""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel, Field, field_validator

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill
from app.llm.skills.profile.weakness_analyze import Weakness


class Suggestion(BaseModel):
    target_dimension: str
    advice: str

    @field_validator("advice")
    @classmethod
    def min_length(cls, v: str) -> str:
        if len(v) < 100:
            raise ValueError("advice 必须 ≥ 100 字符")
        return v


class AdviceInput(BaseModel):
    weaknesses: list[Weakness]


class AdviceOutput(BaseModel):
    suggestions: list[Suggestion] = Field(default_factory=list)


class LearningAdviceSkill(Skill[AdviceInput, AdviceOutput]):
    name: ClassVar[str] = "profile.learning_advice"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "profile"
    temperature: ClassVar[float] = 0.2
    input_schema: ClassVar[type[BaseModel]] = AdviceInput
    output_schema: ClassVar[type[BaseModel]] = AdviceOutput

    def render_prompt(self, input_: AdviceInput) -> list[LLMMessage]:
        sys = (
            "你是学习教练。针对薄弱维度生成具体可执行的学习建议（每条 ≥100 字）。"
            "禁止泛泛而谈，需给出具体资源、练习方法、检验标准。"
            "只输出严格 JSON。"
        )
        rows = "\n".join(
            f"- {w.dimension_name} (mastery={w.mastery_score}): {w.reason}"
            for w in input_.weaknesses
        )
        user = (
            f"# 薄弱维度\n{rows}\n\n"
            "# 输出 JSON\n"
            '{"suggestions":[{"target_dimension":"...","advice":"≥100字..."}]}'
        )
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]
