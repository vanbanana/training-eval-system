"""chat.student_qa Skill - Epic 22.3.

简化设计：output 仅 content 字符串，主要用于 system prompt 构造。
真正流式输出由 ChatService 处理。
"""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class StudentQaInput(BaseModel):
    student_name: str = ""
    task_name: str = ""
    user_question: str


class StudentQaOutput(BaseModel):
    content: str


class StudentQaSkill(Skill[StudentQaInput, StudentQaOutput]):
    name: ClassVar[str] = "chat.student_qa"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "chat"
    temperature: ClassVar[float] = 0.5
    input_schema: ClassVar[type[BaseModel]] = StudentQaInput
    output_schema: ClassVar[type[BaseModel]] = StudentQaOutput

    def render_prompt(self, input_: StudentQaInput) -> list[LLMMessage]:
        sys = (
            f"你是 {input_.student_name} 的实训学习助教，"
            f"任务：{input_.task_name}。"
            "回答应该针对学生本次提交的具体问题；"
            "如需查看具体数据请使用工具，不要编造数字。"
            "回答简洁、直接、可执行。"
        )
        user = input_.user_question
        return [
            LLMMessage(role="system", content=sys),
            LLMMessage(role="user", content=user),
        ]
