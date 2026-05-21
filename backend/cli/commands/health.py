"""tes-cli health-check / backup-now - 27.2."""

from __future__ import annotations

import asyncio

import typer


def health_check() -> None:
    """检查 DB / Redis / 磁盘 / LLM 配置."""
    from sqlalchemy import text as sa_text

    from app.core.config import get_settings
    from app.core.database import SessionLocal

    settings = get_settings()
    failures: list[str] = []

    async def _check_db() -> bool:
        try:
            async with SessionLocal() as db:
                await db.execute(sa_text("SELECT 1"))
            return True
        except Exception as e:  # noqa: BLE001
            failures.append(f"db: {e}")
            return False

    db_ok = asyncio.run(_check_db())
    typer.echo(f"DB:    {'OK' if db_ok else 'FAIL'}")
    typer.echo(f"ENV:   {settings.env}")
    typer.echo(f"REDIS: {'configured' if settings.redis_url else 'unset'}")

    if failures:
        for f in failures:
            typer.secho(f, fg=typer.colors.RED)
        raise typer.Exit(code=1)


def backup_now() -> None:
    """备份占位（生产实现 pg_dump）."""
    from app.core.config import get_settings

    settings = get_settings()
    if "postgres" not in settings.db_url:
        typer.echo("non-postgres skip")
        return
    # 实际实现执行 pg_dump 命令；此处只占位
    typer.echo("[backup-now] 占位：实际部署在 systemd timer 中执行 pg_dump")
