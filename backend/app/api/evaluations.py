"""评价路由 - Epic 16.9."""

from __future__ import annotations

import random

from fastapi import APIRouter
from sqlalchemy import select
from sqlalchemy.orm import selectinload

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    ResourceNotFoundError,
)
from app.models.evaluation import (
    DimensionScore,
    Evaluation,
    EvaluationHistory,
)
from app.models.task import Dimension, TrainingTask
from app.models.upload import Upload
from app.schemas.evaluation import (
    BulkActionRequest,
    DimensionScoreUpdate,
    EvaluationHistoryOut,
    EvaluationOut,
)
from app.services.evaluation_service import EvaluationService

router = APIRouter(prefix="/api/evaluations", tags=["evaluations"])


def _to_out(ev: Evaluation) -> EvaluationOut:
    return EvaluationOut(
        id=ev.id,
        task_id=ev.task_id,
        student_id=ev.student_id,
        upload_id=ev.upload_id,
        status=ev.status,
        total_score=ev.total_score,
        teacher_comment=ev.teacher_comment,
        created_at=ev.created_at,
        scores=[
            {
                "dimension_id": s.dimension_id,
                "ai_score": s.ai_score,
                "teacher_score": s.teacher_score,
                "rationale": s.rationale,
            }
            for s in ev.scores
        ],
    )


@router.post("/trigger/{upload_id}", status_code=201)
async def trigger_evaluation(
    upload_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    """触发评价（dev 模式 mock 随机分代替 LLM）."""
    upload = await db.get(Upload, upload_id)
    if not upload:
        raise ResourceNotFoundError("upload not found")
    if upload.student_id != current.id and current.role not in ("teacher", "admin"):
        raise AuthorizationError("无权操作此提交")

    task = await db.get(TrainingTask, upload.task_id)
    if not task:
        raise ResourceNotFoundError("task not found")

    existing = (
        await db.execute(select(Evaluation).where(Evaluation.upload_id == upload_id))
    ).scalar_one_or_none()
    if existing:
        raise BusinessRuleError("该提交已有评价记录", field="upload_id")

    dims = (
        await db.execute(select(Dimension).where(Dimension.task_id == task.id))
    ).scalars().all()
    if not dims:
        raise BusinessRuleError("任务无评分维度", field="dimensions")

    submission_text = (
        f"文件：{upload.filename}，大小：{upload.file_size} 字节。（dev mock）"
    )

    from app.llm.scoring import score_submission

    scores_data = await score_submission(
        db,
        dimensions=list(dims),
        submission_text=submission_text,
        task_description=task.description + "\n" + task.requirements,
    )

    evaluation = Evaluation(
        task_id=task.id,
        student_id=upload.student_id,
        upload_id=upload_id,
        status="auto_scored",
    )
    db.add(evaluation)
    await db.flush()

    total = 0.0
    for dim in dims:
        score_item = next(
            (s for s in scores_data if s.get("dimension_id") == dim.id), None
        )
        score = (
            float(score_item["score"])
            if score_item and "score" in score_item
            else round(random.uniform(60, 95), 1)
        )
        rationale = (
            score_item.get("rationale", f"{dim.name} 评分")
            if score_item
            else f"{dim.name} 评分"
        )
        total += score * dim.weight / 100
        db.add(
            DimensionScore(
                evaluation_id=evaluation.id,
                dimension_id=dim.id,
                ai_score=score,
                rationale=rationale,
            )
        )
    evaluation.total_score = round(total, 1)
    db.add(
        EvaluationHistory(
            evaluation_id=evaluation.id,
            operator_id=None,
            action="auto_score",
            before_value=None,
            after_value={"total_score": evaluation.total_score},
        )
    )
    await db.commit()
    await db.refresh(evaluation, ["scores"])
    return {
        "evaluation_id": evaluation.id,
        "total_score": evaluation.total_score,
        "status": evaluation.status,
    }


@router.get("/my", response_model=list[EvaluationOut])
async def my_evaluations(db: DbSession, current: CurrentUser) -> list[EvaluationOut]:
    stmt = (
        select(Evaluation)
        .options(selectinload(Evaluation.scores))
        .where(Evaluation.student_id == current.id)
        .order_by(Evaluation.created_at.desc())
    )
    evals = (await db.execute(stmt)).scalars().all()
    return [_to_out(e) for e in evals]


@router.get("/{eval_id}", response_model=EvaluationOut)
async def get_evaluation(
    eval_id: int, db: DbSession, current: CurrentUser
) -> EvaluationOut:
    ev = (
        await db.execute(
            select(Evaluation)
            .options(selectinload(Evaluation.scores))
            .where(Evaluation.id == eval_id)
        )
    ).scalar_one_or_none()
    if ev is None:
        raise ResourceNotFoundError("evaluation not found")
    if current.role == "student" and ev.student_id != current.id:
        raise AuthorizationError("无权查看")
    return _to_out(ev)


@router.patch("/{eval_id}/dimensions/{dim_id}", response_model=EvaluationOut)
async def update_dimension(
    eval_id: int,
    dim_id: int,
    payload: DimensionScoreUpdate,
    db: DbSession,
    current: CurrentUser,
) -> EvaluationOut:
    if current.role == "student":
        raise AuthorizationError("仅教师可修改主观分")
    svc = EvaluationService()
    ev = await svc.update_dimension_subjective(
        db,
        evaluation_id=eval_id,
        dimension_id=dim_id,
        subj_score=payload.subj_score,
        comment=payload.comment,
        operator=current,
    )
    await db.commit()
    return _to_out(ev)


@router.post("/bulk-action")
async def bulk_action(
    payload: BulkActionRequest, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    if current.role == "student":
        raise AuthorizationError("仅教师可批量操作")
    svc = EvaluationService()
    out = await svc.bulk_action(
        db,
        evaluation_ids=payload.evaluation_ids,
        action=payload.action,
        operator=current,
        reason=payload.reason,
    )
    await db.commit()
    return out


@router.get("/{eval_id}/history", response_model=list[EvaluationHistoryOut])
async def get_history(
    eval_id: int, db: DbSession, current: CurrentUser
) -> list[EvaluationHistoryOut]:
    if current.role == "student":
        raise AuthorizationError("仅教师可查看历史")
    rows = list(
        (
            await db.execute(
                select(EvaluationHistory)
                .where(EvaluationHistory.evaluation_id == eval_id)
                .order_by(EvaluationHistory.changed_at)
            )
        )
        .scalars()
        .all()
    )
    return [
        EvaluationHistoryOut(
            id=h.id,
            evaluation_id=h.evaluation_id,
            operator_id=h.operator_id,
            action=h.action,
            before_value=h.before_value,
            after_value=h.after_value,
            changed_at=h.changed_at,
        )
        for h in rows
    ]


@router.get("/by-task/{task_id}", response_model=list[EvaluationOut])
async def list_for_task(
    task_id: int,
    db: DbSession,
    current: CurrentUser,
    status: str | None = None,
    min_score: float | None = None,
    max_score: float | None = None,
) -> list[EvaluationOut]:
    if current.role == "student":
        raise AuthorizationError("仅教师可查看")
    stmt = (
        select(Evaluation)
        .options(selectinload(Evaluation.scores))
        .where(Evaluation.task_id == task_id)
    )
    if status:
        stmt = stmt.where(Evaluation.status == status)
    if min_score is not None:
        stmt = stmt.where(Evaluation.total_score >= min_score)
    if max_score is not None:
        stmt = stmt.where(Evaluation.total_score <= max_score)
    rows = list((await db.execute(stmt)).scalars().all())
    return [_to_out(e) for e in rows]
