"""tes-cli 入口 - Epic 27.1."""

from __future__ import annotations

import typer

from cli.commands import audit, db, health, seed, simulate, skill_eval


app = typer.Typer(
    name="tes-cli",
    help="智能实训评价管理系统 - 运维与验证工具",
    no_args_is_help=True,
)

# 子命令分组
db_app = typer.Typer(help="数据库相关")
skill_app = typer.Typer(help="Skill 评估")
audit_app = typer.Typer(help="审计日志")
celery_app = typer.Typer(help="Celery 操作")

app.add_typer(db_app, name="db")
app.add_typer(skill_app, name="skill")
app.add_typer(audit_app, name="audit")
app.add_typer(celery_app, name="celery")


@app.callback()
def _root(
    env: str = typer.Option(
        "dev", "--env", help="运行环境 dev/test/prod"
    ),
    verbose: bool = typer.Option(False, "--verbose", "-v"),
) -> None:
    """全局选项."""
    if verbose:
        typer.echo(f"[tes-cli] env={env}")


# ============== 顶层命令注册 ==============

app.command("seed")(seed.seed)
app.command("health-check")(health.health_check)
app.command("simulate-evaluation")(simulate.simulate_evaluation)
app.command("backup-now")(health.backup_now)


# ============== skill 子命令 ==============

skill_app.command("eval")(skill_eval.skill_eval_cmd)


# ============== audit 子命令 ==============

audit_app.command("archive")(audit.archive)


# ============== db 子命令 ==============

db_app.command("migrate")(db.migrate)
db_app.command("seed-system")(db.seed_system)


if __name__ == "__main__":
    app()
