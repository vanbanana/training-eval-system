"""Epic 16.3 验收：score.dimension Skill."""

from __future__ import annotations

import pytest

from app.llm.base import LLMResponse
from app.llm.skills.base import SkillOutputError
from app.llm.skills.score import (
    DimensionInfo,
    DimensionScoreInput,
    DimensionScoreSkill,
)
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


def _input() -> DimensionScoreInput:
    return DimensionScoreInput(
        task_requirements="完成完整的 CRUD 实训",
        dimension=DimensionInfo(name="代码质量", description="可读性与规范"),
        parse_summary="实现完整 CRUD",
    )


class TestDimensionScoreSkill:
    def test_metadata(self) -> None:
        s = DimensionScoreSkill()
        assert s.name == "score.dimension"
        assert s.version == "1.0.0"

    async def test_happy_path(self) -> None:
        """Given 合法 score+rationale When execute Then 解析正常."""
        skill = DimensionScoreSkill()
        fake = FakeLLM()
        rationale = (
            "代码风格整体规范，命名清晰且语义明确；"
            "但部分函数缺少类型注解，建议补全以提升可维护性。"
            "测试覆盖率良好，整体质量较高。"
        )
        fake._default = LLMResponse(
            content=f'{{"score": 85, "rationale": "{rationale}"}}'
        )
        out = await skill.execute(_input(), fake)
        assert out.score == 85
        assert len(out.rationale) >= 50

    async def test_short_rationale_fails(self) -> None:
        """Given rationale < 50 字符 When execute Then SkillOutputError."""
        skill = DimensionScoreSkill()
        fake = FakeLLM()
        fake._default = LLMResponse(
            content='{"score": 85, "rationale": "不错"}'
        )
        with pytest.raises(SkillOutputError):
            await skill.execute(_input(), fake)

    async def test_score_out_of_range_fails(self) -> None:
        """Given score=101 When execute Then SkillOutputError."""
        skill = DimensionScoreSkill()
        fake = FakeLLM()
        rat = "理由：" + "测试" * 20
        fake._default = LLMResponse(
            content=f'{{"score": 101, "rationale": "{rat}"}}'
        )
        with pytest.raises(SkillOutputError):
            await skill.execute(_input(), fake)
