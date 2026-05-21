"""Skill 抽象基类 - 渲染 prompt → 调 LLM → 解析输出 → 重试."""

from __future__ import annotations

import json
import re
from abc import ABC, abstractmethod
from typing import ClassVar, Generic, TypeVar

from pydantic import BaseModel, ValidationError

from app.core.exceptions import BusinessRuleError
from app.core.logging import get_logger
from app.llm.base import LLMMessage, LLMProvider, LLMResponse


log = get_logger(__name__)


class SkillOutputError(BusinessRuleError):
    """LLM 输出无法通过 schema 校验或 JSON 解析."""

    error_code = "SKILL_OUTPUT_INVALID"


InputT = TypeVar("InputT", bound=BaseModel)
OutputT = TypeVar("OutputT", bound=BaseModel)


class Skill(ABC, Generic[InputT, OutputT]):
    """Skill 抽象基类.

    子类必须设置 ClassVar：name / version / category.
    必须实现 input_schema / output_schema / render_prompt.
    可选实现 system_prompt（默认空）.
    """

    name: ClassVar[str] = ""
    version: ClassVar[str] = ""
    category: ClassVar[str] = "default"
    temperature: ClassVar[float] = 0.3
    max_tokens: ClassVar[int | None] = None
    max_retries: ClassVar[int] = 3

    input_schema: ClassVar[type[BaseModel]]
    output_schema: ClassVar[type[BaseModel]]

    def __init_subclass__(cls, **kwargs: object) -> None:
        super().__init_subclass__(**kwargs)
        # 仅校验"已经被设计为可实例化"的具体子类
        if getattr(cls, "__abstractmethods__", None):
            return
        if not cls.name:
            raise ValueError(f"{cls.__name__}.name 必须设置")
        if not cls.version:
            raise ValueError(f"{cls.__name__}.version 必须设置")

    # ============ 抽象 ============

    @abstractmethod
    def render_prompt(self, input_: InputT) -> list[LLMMessage]:
        """子类实现：将输入渲染为消息列表."""

    # ============ 容错 JSON 解析 ============

    @staticmethod
    def extract_json(text: str) -> dict[str, object]:
        """从可能含 markdown 代码块或散文的文本中提取 JSON 对象."""
        # 1. 直接尝试整段 parse
        try:
            return json.loads(text)
        except (json.JSONDecodeError, TypeError):
            pass

        # 2. 提取 ```json ... ``` 块
        match = re.search(r"```(?:json)?\s*([\s\S]*?)```", text)
        if match:
            inner = match.group(1).strip()
            try:
                return json.loads(inner)
            except json.JSONDecodeError:
                pass

        # 3. 提取第一个 {...} 平衡对
        depth = 0
        start = -1
        for i, ch in enumerate(text):
            if ch == "{":
                if depth == 0:
                    start = i
                depth += 1
            elif ch == "}":
                depth -= 1
                if depth == 0 and start >= 0:
                    candidate = text[start : i + 1]
                    try:
                        return json.loads(candidate)
                    except json.JSONDecodeError:
                        pass
        raise SkillOutputError("无法从 LLM 输出提取 JSON")

    def parse_output(self, raw: str) -> OutputT:
        data = self.extract_json(raw)
        try:
            return self.output_schema(**data)  # type: ignore[return-value]
        except ValidationError as e:
            raise SkillOutputError(f"输出 schema 校验失败: {e}") from e

    # ============ 主入口 ============

    async def execute(self, input_: InputT, llm: LLMProvider) -> OutputT:
        """渲染 → 调 LLM → 解析；失败重试 max_retries 次."""
        messages = self.render_prompt(input_)
        last_error: Exception | None = None
        for attempt in range(1, self.max_retries + 1):
            log.info(
                "skill.execute.start",
                skill=self.name,
                version=self.version,
                attempt=attempt,
            )
            try:
                response: LLMResponse = await llm.chat(
                    messages,
                    temperature=self.temperature,
                    max_tokens=self.max_tokens,
                )
                output = self.parse_output(response.content)
                log.info(
                    "skill.execute.success",
                    skill=self.name,
                    attempt=attempt,
                )
                return output
            except SkillOutputError as e:
                last_error = e
                log.warning(
                    "skill.execute.parse_failed",
                    skill=self.name,
                    attempt=attempt,
                    error=str(e),
                )
        # 重试用尽
        log.error("skill.execute.failed", skill=self.name, error=str(last_error))
        raise SkillOutputError(
            f"Skill {self.name} 重试 {self.max_retries} 次后仍失败: {last_error}"
        )
