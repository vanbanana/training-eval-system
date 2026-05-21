"""教师批改路由."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError, BusinessRuleError, ResourceNotFoundError
from app.models.evaluation import Evaluation
from app.models.task import TrainingTask
from app.models.upload import Upload
from app.models.user import User

router = APIRouter(prefix="/api/grading", tags=["grading"])


@router.get("/tasks/{task_id}/submissions")
async def list_submissions(task_id: int, db: DbSession, current: CurrentUser) -> list[dict[str, object]]:
    """教师查看某任务下所有学生提交 + 评价状态."""
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可访问")
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise ResourceNotFoundError("task not found")
    if current.role == "teacher" and task.teacher_id != current.id:
        raise AuthorizationError("无权查看他人任务")

    uploads = (
        await db.execute(
            select(Upload).where(Upload.task_id == task_id).order_by(Upload.created_at.desc())
        )
    ).scalars().all()

    result = []
    for u in uploads:
        student = await db.get(User, u.student_id)
        eval_obj = (
            await db.execute(select(Evaluation).where(Evaluation.upload_id == u.id))
        ).scalar_one_or_none()
        result.append({
            "upload_id": u.id,
            "student_id": u.student_id,
            "student_name": student.display_name if student else "未知",
            "filename": u.filename,
            "file_size": u.file_size,
            "version": u.version,
            "parse_status": u.parse_status,
            "uploaded_at": u.created_at.isoformat(),
            "evaluation_id": eval_obj.id if eval_obj else None,
            "eval_status": eval_obj.status if eval_obj else None,
            "total_score": eval_obj.total_score if eval_obj else None,
        })
    return result


class ConfirmScoreRequest(BaseModel):
    teacher_comment: str = ""
    score_overrides: dict[int, float] = Field(default_factory=dict, description="dimension_id -> teacher_score")


@router.post("/evaluations/{eval_id}/confirm")
async def confirm_evaluation(
    eval_id: int, payload: ConfirmScoreRequest, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    """教师确认评价（可调分）."""
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可操作")
    evaluation = await db.get(Evaluation, eval_id)
    if not evaluation:
        raise ResourceNotFoundError("evaluation not found")
    if evaluation.status == "confirmed":
        raise BusinessRuleError("已确认，不可重复操作")

    # 应用教师调分
    for ds in evaluation.scores:
        if ds.dimension_id in payload.score_overrides:
            ds.teacher_score = payload.score_overrides[ds.dimension_id]

    evaluation.teacher_comment = payload.teacher_comment
    evaluation.status = "confirmed"

    # 重算总分（有教师分用教师分，否则用 AI 分）
    from app.models.task import Dimension
    dims = (await db.execute(select(Dimension).where(Dimension.task_id == evaluation.task_id))).scalars().all()
    dim_map = {d.id: d.weight for d in dims}
    total = 0.0
    for ds in evaluation.scores:
        score = ds.teacher_score if ds.teacher_score is not None else (ds.ai_score or 0)
        total += score * dim_map.get(ds.dimension_id, 0) / 100
    evaluation.total_score = round(total, 1)

    await db.commit()
    return {"evaluation_id": eval_id, "status": "confirmed", "total_score": evaluation.total_score}


@router.post("/evaluations/{eval_id}/reject")
async def reject_evaluation(
    eval_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    """教师打回重做."""
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可操作")
    evaluation = await db.get(Evaluation, eval_id)
    if not evaluation:
        raise ResourceNotFoundError("evaluation not found")
    evaluation.status = "rejected"
    await db.commit()
    return {"evaluation_id": eval_id, "status": "rejected"}
