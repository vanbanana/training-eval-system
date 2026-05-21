"""ReportService - Epic 20.4."""

from __future__ import annotations

from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.models.evaluation import Evaluation
from app.models.task import Dimension, TrainingTask
from app.models.user import User
from app.reporting import (
    render_personal_pdf,
    render_statistics_xlsx,
)


log = get_logger(__name__)


NO_EVALUATION_DATA_ERROR_CODE = "NO_EVALUATION_DATA"


class NoEvaluationDataError(BusinessRuleError):
    error_code = NO_EVALUATION_DATA_ERROR_CODE


class ReportService:
    async def generate_personal_pdf(
        self,
        db: AsyncSession,
        *,
        evaluation_id: int,
        operator: User,
    ) -> tuple[bytes, str]:
        """返回 (pdf 字节, 文件名)."""
        ev = (
            await db.execute(
                select(Evaluation)
                .options(selectinload(Evaluation.scores))
                .where(Evaluation.id == evaluation_id)
            )
        ).scalar_one_or_none()
        if ev is None:
            raise ResourceNotFoundError(f"evaluation {evaluation_id} not found")
        if operator.role == "student" and operator.id != ev.student_id:
            raise AuthorizationError("无权下载他人报告")

        student = await db.get(User, ev.student_id)
        task = await db.get(TrainingTask, ev.task_id)
        dim_ids = {s.dimension_id for s in ev.scores}
        dim_map: dict[int, str] = {}
        if dim_ids:
            for d in (
                await db.execute(
                    select(Dimension).where(Dimension.id.in_(dim_ids))
                )
            ).scalars():
                dim_map[d.id] = d.name

        data = {
            "student_name": student.display_name if student else "",
            "task_name": task.name if task else "",
            "total_score": ev.total_score,
            "teacher_comment": ev.teacher_comment,
            "dimensions": [
                {
                    "name": dim_map.get(s.dimension_id, f"维度{s.dimension_id}"),
                    "ai_score": s.ai_score,
                    "teacher_score": s.teacher_score,
                    "rationale": s.rationale,
                }
                for s in ev.scores
            ],
        }
        pdf = render_personal_pdf(data)
        filename = f"report_{student.username if student else 'student'}_{ev.id}.pdf"
        return pdf, filename

    async def generate_statistics_xlsx(
        self,
        db: AsyncSession,
        *,
        task_id: int,
        operator: User,
    ) -> tuple[bytes, str]:
        if operator.role == "student":
            raise AuthorizationError("仅教师可导出")
        task = await db.get(TrainingTask, task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {task_id} not found")
        if (
            operator.role == "teacher"
            and task.teacher_id != operator.id
        ):
            raise AuthorizationError("无权导出其他教师任务")

        evals = list(
            (
                await db.execute(
                    select(Evaluation)
                    .options(selectinload(Evaluation.scores))
                    .where(Evaluation.task_id == task_id)
                )
            )
            .scalars()
            .all()
        )
        if not evals:
            raise NoEvaluationDataError("该任务尚无评价数据")

        dim_ids = {s.dimension_id for ev in evals for s in ev.scores}
        dim_map: dict[int, str] = {}
        for d in (
            await db.execute(
                select(Dimension).where(Dimension.id.in_(dim_ids))
            )
        ).scalars():
            dim_map[d.id] = d.name

        student_ids = {ev.student_id for ev in evals}
        student_map: dict[int, str] = {}
        for u in (
            await db.execute(select(User).where(User.id.in_(student_ids)))
        ).scalars():
            student_map[u.id] = u.display_name

        rows: list[dict[str, Any]] = []
        for ev in evals:
            rows.append(
                {
                    "student_name": student_map.get(ev.student_id, "?"),
                    "total_score": ev.total_score,
                    "status": ev.status,
                    "dimensions": [
                        {
                            "name": dim_map.get(
                                s.dimension_id, f"维度{s.dimension_id}"
                            ),
                            "ai_score": s.ai_score,
                            "teacher_score": s.teacher_score,
                        }
                        for s in ev.scores
                    ],
                }
            )

        xlsx = render_statistics_xlsx(rows)
        filename = f"statistics_task_{task_id}.xlsx"
        return xlsx, filename
