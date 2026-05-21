"""tes-cli skill eval - 27.4."""

from __future__ import annotations

import json
from pathlib import Path

import typer


def skill_eval_cmd(
    skill: str = typer.Option(..., "--skill"),
    version: str = typer.Option(..., "--version"),
    llm: str = typer.Option("fake", "--llm"),
) -> None:
    """运行 Golden Set 评估（仅 fake LLM 在此 CLI 内可直接跑）."""
    if llm != "fake":
        typer.echo(f"目前只支持 --llm=fake；不支持 {llm}")
        raise typer.Exit(code=2)

    base = Path("tests/skills/golden")
    case_file = (
        base
        / skill.replace(".", "/").split("/", 1)[0]
        / f"{skill.split('.', 1)[1]}_cases.json"
    )
    if not case_file.exists():
        typer.echo(f"未找到 case 文件: {case_file}")
        raise typer.Exit(code=2)

    cases = json.loads(case_file.read_text(encoding="utf-8"))
    typer.echo(
        f"[skill={skill}@{version}] cases={len(cases)} llm={llm}"
    )
    typer.echo("（运行实际验证请使用 pytest tests/skills/）")
