"""Epic 15.1 验收：verify.coverage_check / verify.logic_audit Skills."""

from __future__ import annotations

import pytest

from app.llm.base import LLMResponse
from app.llm.skills.base import SkillOutputError
from app.llm.skills.verify import (
    CoverageCheckSkill,
    CoverageInput,
    LogicAuditInput,
    LogicAuditSkill,
)
from tests.factories import faker
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


class TestCoverageCheckSkill:
    """Skill 注册元数据."""

    def test_metadata(self) -> None:
        s = CoverageCheckSkill()
        assert s.name == "verify.coverage_check"
        assert s.version == "1.0.0"
        assert s.category == "verify"

    def test_render_prompt_includes_inputs(self) -> None:
        # Given
        skill = CoverageCheckSkill()
        req = "1. " + faker.sentence() + "\n2. " + faker.sentence()
        inp = CoverageInput(
            task_requirements=req,
            parse_summary="提交内容摘要",
            parse_key_points=["要点1", "要点2"],
        )
        # When
        msgs = skill.render_prompt(inp)
        # Then
        joined = "\n".join(m.content for m in msgs)
        assert req in joined
        assert "要点1" in joined
        assert msgs[0].role == "system"

    async def test_execute_happy_path(self) -> None:
        """Given 合法 JSON 输出 When execute Then 解析为 CoverageOutput."""
        skill = CoverageCheckSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(
            content=(
                '{"checkpoints":[{"requirement":"r1","matched":true,'
                '"evidence":"e","confidence":85},'
                '{"requirement":"r2","matched":false,"evidence":"",'
                '"confidence":40}]}'
            )
        )
        out = await skill.execute(
            CoverageInput(task_requirements="1. r1\n2. r2"), fake
        )
        assert len(out.checkpoints) == 2
        assert out.checkpoints[0].matched is True
        assert out.checkpoints[1].confidence == 40

    async def test_execute_invalid_confidence_fails(self) -> None:
        """Given confidence 越界 When execute Then SkillOutputError."""
        skill = CoverageCheckSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(
            content=(
                '{"checkpoints":[{"requirement":"r","matched":true,'
                '"evidence":"","confidence":150}]}'
            )
        )
        with pytest.raises(SkillOutputError):
            await skill.execute(
                CoverageInput(task_requirements="1. r"), fake
            )

    async def test_execute_empty_checkpoints_fails(self) -> None:
        """Given 空 checkpoints When execute Then SkillOutputError."""
        skill = CoverageCheckSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(content='{"checkpoints":[]}')
        with pytest.raises(SkillOutputError):
            await skill.execute(
                CoverageInput(task_requirements="1. r"), fake
            )


class TestLogicAuditSkill:
    def test_metadata(self) -> None:
        s = LogicAuditSkill()
        assert s.name == "verify.logic_audit"
        assert s.version == "1.0.0"

    async def test_execute_happy_path(self) -> None:
        """Given 含 issues 的合法 JSON When execute Then 解析正常."""
        skill = LogicAuditSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(
            content=(
                '{"issues":[{"description":"前后矛盾",'
                '"severity":"high","location":"第3章"}]}'
            )
        )
        out = await skill.execute(
            LogicAuditInput(task_requirements="x", parse_summary="y"), fake
        )
        assert len(out.issues) == 1
        assert out.issues[0].severity == "high"

    async def test_execute_no_issues(self) -> None:
        """Given 空 issues When execute Then 返回空列表（合法）."""
        skill = LogicAuditSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(content='{"issues":[]}')
        out = await skill.execute(
            LogicAuditInput(task_requirements="x"), fake
        )
        assert out.issues == []

    async def test_execute_invalid_severity_fails(self) -> None:
        """Given severity 不在枚举 When execute Then SkillOutputError."""
        skill = LogicAuditSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(
            content=(
                '{"issues":[{"description":"x","severity":"critical","location":""}]}'
            )
        )
        with pytest.raises(SkillOutputError):
            await skill.execute(
                LogicAuditInput(task_requirements="x"), fake
            )
