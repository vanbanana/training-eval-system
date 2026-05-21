"""Task 11.1 / 11.2 / 11.3 验收：Skill 框架 + Registry."""

from __future__ import annotations

import pytest
from pydantic import BaseModel

from app.llm.base import LLMMessage, LLMResponse
from app.llm.skills import SkillOutputError, registry
from app.llm.skills.base import Skill
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


class _DummyInput(BaseModel):
    text: str


class _DummyOutput(BaseModel):
    score: int
    reason: str


class _DummySkill(Skill[_DummyInput, _DummyOutput]):
    name = "test.dummy"
    version = "1.0"
    category = "test"
    input_schema = _DummyInput
    output_schema = _DummyOutput

    def render_prompt(self, input_: _DummyInput) -> list[LLMMessage]:
        return [
            LLMMessage(role="system", content="返回 JSON {score, reason}"),
            LLMMessage(role="user", content=input_.text),
        ]


class TestSubclassValidation:
    def test_subclass_must_set_name(self) -> None:
        with pytest.raises(ValueError, match="name"):

            class _MissingName(Skill):
                name = ""
                version = "1.0"
                input_schema = _DummyInput
                output_schema = _DummyOutput

                def render_prompt(self, input_: _DummyInput) -> list[LLMMessage]:
                    return []

    def test_subclass_must_set_version(self) -> None:
        with pytest.raises(ValueError, match="version"):

            class _MissingVersion(Skill):
                name = "x"
                version = ""
                input_schema = _DummyInput
                output_schema = _DummyOutput

                def render_prompt(self, input_: _DummyInput) -> list[LLMMessage]:
                    return []


class TestExtractJson:
    def test_pure_json(self) -> None:
        assert _DummySkill.extract_json('{"a": 1}') == {"a": 1}

    def test_markdown_code_fence(self) -> None:
        text = "Here is the result:\n```json\n{\"score\": 90}\n```\n"
        assert _DummySkill.extract_json(text) == {"score": 90}

    def test_embedded_in_prose(self) -> None:
        text = "评分如下：{\"score\": 80, \"reason\": \"good\"} 完成。"
        assert _DummySkill.extract_json(text)["score"] == 80

    def test_unparseable_raises(self) -> None:
        with pytest.raises(SkillOutputError):
            _DummySkill.extract_json("just plain text")


class TestExecute:
    async def test_happy_path(self) -> None:
        skill = _DummySkill()
        fake = FakeLLM(
            default=LLMResponse(content='{"score": 88, "reason": "good"}')
        )
        result = await skill.execute(_DummyInput(text="hello"), fake)
        assert result.score == 88
        assert result.reason == "good"

    async def test_retries_3_then_fails(self) -> None:
        """LLM 持续返回非 JSON，3 次重试后抛 SkillOutputError."""
        skill = _DummySkill()
        fake = FakeLLM(default=LLMResponse(content="not json at all"))
        with pytest.raises(SkillOutputError):
            await skill.execute(_DummyInput(text="x"), fake)
        # 重试了 3 次
        assert len(fake.calls) == 3

    async def test_recovers_after_initial_failure(self) -> None:
        """第 1、2 次返回非 JSON，第 3 次返回合法；最终成功."""
        skill = _DummySkill()
        fake = FakeLLM()
        # FakeLLM 默认会持续返回 default；用 deque 模拟不同响应
        responses = iter(
            [
                LLMResponse(content="rubbish"),
                LLMResponse(content="still rubbish"),
                LLMResponse(content='{"score": 70, "reason": "ok"}'),
            ]
        )

        async def _chat(messages, **kw):  # type: ignore[no-untyped-def]
            fake.calls.append(list(messages))
            return next(responses)

        fake.chat = _chat  # type: ignore[method-assign]
        result = await skill.execute(_DummyInput(text="x"), fake)
        assert result.score == 70
        assert len(fake.calls) == 3


class TestRegistry:
    def test_register_and_get(self) -> None:
        registry.clear()
        registry.register(_DummySkill)
        instance = registry.get("test.dummy")
        assert isinstance(instance, _DummySkill)

    def test_versioned_lookup(self) -> None:
        registry.clear()
        registry.register(_DummySkill)
        instance = registry.get("test.dummy", version="1.0")
        assert instance.version == "1.0"

    def test_unknown_raises(self) -> None:
        registry.clear()
        with pytest.raises(KeyError):
            registry.get("nope")

    def test_list_all(self) -> None:
        registry.clear()
        registry.register(_DummySkill)
        names = registry.list_names()
        assert "test.dummy" in names
