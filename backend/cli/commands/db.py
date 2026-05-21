"""tes-cli db migrate / seed-system - 27.6."""

from __future__ import annotations

import asyncio
import subprocess
import sys

import typer


def migrate() -> None:
    """alembic upgrade head."""
    proc = subprocess.run(
        [sys.executable, "-m", "alembic", "upgrade", "head"],
        check=False,
    )
    raise typer.Exit(code=proc.returncode)


def seed_system() -> None:
    """填充 system_config 默认 + 系统预置评价模板."""

    async def _run() -> dict[str, int]:
        from sqlalchemy import select

        from app.core.database import SessionLocal
        from app.models.system_config import SystemConfig
        from app.models.template import EvalTemplate, TemplateDimension

        defaults = {
            "evaluation.objective_ratio": "0.6",
            "upload.max_size_mb": "50",
            "similarity.hamming_threshold": "6",
            "similarity.cosine_threshold": "0.80",
            "chat.daily_quota": "50",
            "chat.max_tool_rounds": "5",
            "rate_limit.login_per_minute": "5",
            "rate_limit.upload_per_hour": "20",
            "rate_limit.chat_per_minute": "10",
            "deadline.reminder_hours": "24",
        }
        config_added = 0
        template_added = 0
        async with SessionLocal() as db:
            for k, v in defaults.items():
                exists = (
                    await db.execute(
                        select(SystemConfig).where(SystemConfig.key == k)
                    )
                ).scalar_one_or_none()
                if exists is None:
                    db.add(SystemConfig(key=k, value=v))
                    config_added += 1
            # 系统预置模板
            preset_templates = [
                ("通用实训评价", "适用于大部分实训"),
                ("代码作业评价", "侧重代码质量"),
                ("文档撰写评价", "侧重文档质量"),
            ]
            for name, desc in preset_templates:
                exists_t = (
                    await db.execute(
                        select(EvalTemplate).where(EvalTemplate.name == name)
                    )
                ).scalar_one_or_none()
                if exists_t is None:
                    t = EvalTemplate(
                        name=name,
                        description=desc,
                        visibility="system",
                        owner_id=None,
                    )
                    db.add(t)
                    await db.flush()
                    db.add(
                        TemplateDimension(
                            template_id=t.id,
                            name="完整性",
                            weight=40,
                            order_index=0,
                        )
                    )
                    db.add(
                        TemplateDimension(
                            template_id=t.id,
                            name="质量",
                            weight=40,
                            order_index=1,
                        )
                    )
                    db.add(
                        TemplateDimension(
                            template_id=t.id,
                            name="规范性",
                            weight=20,
                            order_index=2,
                        )
                    )
                    template_added += 1
            await db.commit()
        return {
            "config_added": config_added,
            "template_added": template_added,
        }

    out = asyncio.run(_run())
    typer.echo(out)
