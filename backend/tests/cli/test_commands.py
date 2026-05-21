"""Epic 27.7 验收：CLI 命令测试."""

from __future__ import annotations

import pytest
from typer.testing import CliRunner

from cli.main import app


pytestmark = pytest.mark.unit


runner = CliRunner()


class TestCliRoot:
    def test_help(self) -> None:
        result = runner.invoke(app, ["--help"])
        assert result.exit_code == 0
        assert "tes-cli" in result.stdout or "Usage" in result.stdout

    def test_unknown_command(self) -> None:
        result = runner.invoke(app, ["totally-unknown-cmd"])
        assert result.exit_code != 0


class TestSeed:
    def test_unknown_scale_exits_nonzero(self) -> None:
        result = runner.invoke(app, ["seed", "--scale", "huge"])
        assert result.exit_code != 0


class TestSkillEval:
    def test_unsupported_llm_exits(self) -> None:
        result = runner.invoke(
            app,
            ["skill", "eval", "--skill", "x.y", "--version", "1.0.0", "--llm", "real"],
        )
        assert result.exit_code != 0

    def test_unknown_skill_exits(self) -> None:
        result = runner.invoke(
            app,
            [
                "skill",
                "eval",
                "--skill",
                "nonexistent.skill",
                "--version",
                "1.0.0",
                "--llm",
                "fake",
            ],
        )
        assert result.exit_code != 0
