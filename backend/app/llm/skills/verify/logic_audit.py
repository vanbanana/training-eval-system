"""verify.logic_audit Skill - 检测提交内容的逻辑漏洞."""

from __future__ import annotations

from typing import ClassVar, Literal

from pydantic import BaseModel, Field

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class LogicIssue(BaseModel):
    description: str
    severity: Literal["low", "medium", "high"]
    location: str = ""


class LogicAuditInput(BaseModel):
    task_requirements: str
    parse_summary: str = ""
    parse_key_points: list[str] = Field(default_factory=list)


class LogicAuditOutput(BaseModel):
    issues: list[LogicIssue] = Field(default_factory=list)


class LogicAuditSkill(Skill[LogicAuditInput, LogicAuditOutput]):
    name: ClassVar[str] = "verify.logic_audit"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "verify"
    temperature: ClassVar[float] = 0.0
    input_schema: ClassVar[type[BaseModel]] = LogicAuditInput
    output_schema: ClassVar[type[BaseModel]] = LogicAuditOutput

    def render_prompt(self, input_: LogicAuditInput) -> list[LLMMessage]:
        sys = (
            "你是严谨的逻辑审计助手。"
            "针对实训提交内容找出逻辑矛盾、推理跳跃、数据不一致等问题。"
            "只输出严格 JSON。无问题时返回空数组。"
        )
        user = (
            f"# 实训要求\n{input_.task_requirements}\n\n"
            f"# 提交摘要\n{input_.parse_summary}\n\n"
            f"# 提交要点\n- "
            + "\n- ".join(input_.parse_key_points or ["(无)"])
            + "\n\n"
            "# 输出 JSON\n"
            '{"issues":[{"description":"...","severity":"medium","location":"..."}]}'
        )
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]
