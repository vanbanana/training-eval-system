"""报表导出路由 - Epic 20.5."""

from __future__ import annotations

import csv
import io

from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError
from app.models.evaluation import Evaluation
from app.models.task import TrainingTask
from app.models.user import User
from app.services.report_service import ReportService

router = APIRouter(prefix="/api/reports", tags=["reports"])


@router.get("/personal/{eval_id}")
async def export_personal_pdf(
    eval_id: int, db: DbSession, current: CurrentUser
) -> StreamingResponse:
    svc = ReportService()
    pdf, filename = await svc.generate_personal_pdf(
        db, evaluation_id=eval_id, operator=current
    )
    return StreamingResponse(
        iter([pdf]),
        media_type="application/pdf",
        headers={
            "Content-Disposition": f'attachment; filename="{filename}"'
        },
    )


@router.get("/statistics/{task_id}")
async def export_statistics_xlsx(
    task_id: int, db: DbSession, current: CurrentUser
) -> StreamingResponse:
    svc = ReportService()
    xlsx, filename = await svc.generate_statistics_xlsx(
        db, task_id=task_id, operator=current
    )
    return StreamingResponse(
        iter([xlsx]),
        media_type=(
            "application/vnd.openxmlformats-officedocument."
            "spreadsheetml.sheet"
        ),
        headers={
            "Content-Disposition": f'attachment; filename="{filename}"'
        },
    )


@router.get("/task/{task_id}/csv")
async def export_task_csv(
    task_id: int, db: DbSession, current: CurrentUser
) -> StreamingResponse:
    """旧版 CSV（保留以兼容前端）."""
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师/管理员可导出")
    task = await db.get(TrainingTask, task_id)
    evals = (
        await db.execute(
            select(Evaluation).where(Evaluation.task_id == task_id)
        )
    ).scalars().all()

    output = io.StringIO()
    writer = csv.writer(output)
    writer.writerow(["任务", task.name if task else f"#{task_id}"])
    writer.writerow(["学生ID", "学生姓名", "综合分", "状态", "评价时间"])
    for e in evals:
        student = await db.get(User, e.student_id)
        writer.writerow(
            [
                e.student_id,
                student.display_name if student else "",
                e.total_score,
                e.status,
                e.created_at.isoformat() if e.created_at else "",
            ]
        )

    output.seek(0)
    safe_name = (task.name if task else f"task_{task_id}").replace(" ", "_")
    filename = f"report_{safe_name}.csv"
    return StreamingResponse(
        iter([output.getvalue()]),
        media_type="text/csv",
        headers={"Content-Disposition": f"attachment; filename={filename}"},
    )
