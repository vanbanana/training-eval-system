"""EvaluationFactory + DimensionScoreFactory - Epic 16."""

from __future__ import annotations

import random
from typing import Any

from app.models.evaluation import DimensionScore, Evaluation
from app.models.task import Dimension, TrainingTask
from app.models.upload import Upload
from app.models.user import User
from sqlalchemy.ext.asyncio import AsyncSession

# 评价状态分布参考 evaluation_service:
#   pending → auto_scored → reviewed → finalized / rejected
# dashboard_service._teacher 计 pending=auto_scored, graded=finalized
EVAL_STATUSES: tuple[str, ...] = (
    "pending",
    "auto_scored",
    "reviewed",
    "finalized",
    "rejected",
)


class EvaluationFactory:
    """生成 Evaluation + 每个维度的 DimensionScore.

    使用方法:
        ev = await EvaluationFactory.create_async(
            session,
            upload=upload,
            task=task,
            student=student,
            status="finalized",
        )
    """

    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        upload: Upload,
        task: TrainingTask,
        student: User | None = None,
        status: str = "auto_scored",
        total_score: float | None = None,
        teacher_comment: str = "",
        dimensions: list[Dimension] | None = None,
        with_scores: bool = True,
        rng: random.Random | None = None,
        **extra: Any,
    ) -> Evaluation:
        if status not in EVAL_STATUSES:
            raise ValueError(f"unknown status {status}")
        rnd = rng or random
        student_id = student.id if student else upload.student_id

        # 计算总分（如未给）
        if total_score is None and status not in ("pending", "rejected"):
            total_score = round(rnd.uniform(60.0, 95.0), 1)

        ev = Evaluation(
            task_id=task.id,
            student_id=student_id,
            upload_id=upload.id,
            status=status,
            total_score=total_score,
            teacher_comment=teacher_comment,
            **extra,
        )
        session.add(ev)
        await session.flush()
        await session.refresh(ev)

        if with_scores and status != "pending":
            dims = dimensions if dimensions is not None else list(task.dimensions)
            for d in dims:
                ai = round(rnd.uniform(55.0, 95.0), 1)
                # finalized / reviewed：教师改过分；rejected/auto_scored：仅 AI 分
                teacher_score: float | None
                if status in ("reviewed", "finalized"):
                    teacher_score = round(
                        max(0.0, min(100.0, ai + rnd.uniform(-5.0, 5.0))), 1
                    )
                else:
                    teacher_score = None
                rationale = (
                    f"AI 评价：{d.name} 表现"
                    + ("良好" if ai >= 80 else "中等" if ai >= 60 else "需提升")
                )
                session.add(
                    DimensionScore(
                        evaluation_id=ev.id,
                        dimension_id=d.id,
                        ai_score=ai,
                        teacher_score=teacher_score,
                        rationale=rationale,
                    )
                )
            await session.flush()
            await session.refresh(ev, ["scores"])
        return ev
