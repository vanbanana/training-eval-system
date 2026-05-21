"""Epic 15.5 验收：核查 Golden Set 通过率."""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from app.llm.base import LLMResponse
from app.llm.skills.verify import CoverageCheckSkill, CoverageInput
from tests.fakes.fake_llm import FakeLLM


pytestmark = pytest.mark.unit


CASES_PATH = (
    Path(__file__).parent
    / "golden"
    / "verify"
    / "coverage_check_cases.json"
)


class TestCoverageGoldenSet:
    @pytest.fixture()
    def cases(self) -> list[dict[str, object]]:
        with CASES_PATH.open("r", encoding="utf-8") as f:
            return json.load(f)

    async def test_all_cases_pass(
        self, cases: list[dict[str, object]]
    ) -> None:
        """Given Golden Set 全部 cases When 用 FakeLLM 跑 Skill Then match_rate 与预期一致."""
        skill = CoverageCheckSkill()
        passed = 0
        for case in cases:
            fake = FakeLLM()
            fake._default = LLMResponse(
                content=json.dumps(case["fake_response"], ensure_ascii=False)
            )
            inp = CoverageInput(**case["input"])  # type: ignore[arg-type]
            out = await skill.execute(inp, fake)
            actual_rate = sum(c.matched for c in out.checkpoints) / max(
                len(out.checkpoints), 1
            )
            expected = case["expected_match_rate"]
            assert (
                abs(actual_rate - expected) < 0.01  # type: ignore[operator]
            ), f"case {case['id']}: actual={actual_rate} expected={expected}"
            passed += 1
        assert passed == len(cases)
