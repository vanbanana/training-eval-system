"""tes-cli simulate-evaluation - 27.3."""

from __future__ import annotations

import typer


def simulate_evaluation(
    task_id: int = typer.Option(..., "--task-id"),
) -> None:
    """端到端演练（占位）.

    完整链路依赖 Celery worker 与 LLM；此命令在 dev 模式下:
    - 列出 task 下所有上传
    - 触发 ScoreEngine.score_upload 逐一同步评分
    """
    import asyncio

    from sqlalchemy import select

    from app.core.database import SessionLocal
    from app.models.upload import Upload
    from app.services.score_engine import ScoreEngine

    async def _run() -> int:
        async with SessionLocal() as db:
            uploads = list(
                (
                    await db.execute(
                        select(Upload).where(Upload.task_id == task_id)
                    )
                )
                .scalars()
                .all()
            )
            engine = ScoreEngine()
            scored = 0
            for u in uploads:
                try:
                    await engine.score_upload(db, upload_id=u.id)
                    scored += 1
                except Exception as e:  # noqa: BLE001
                    typer.echo(f"upload {u.id} failed: {e}")
            await db.commit()
            return scored

    n = asyncio.run(_run())
    typer.echo(f"simulated evaluations: {n}")
