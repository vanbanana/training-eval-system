"""相似度检测路由 - Epic 17.6."""

from __future__ import annotations

from datetime import UTC, datetime

from fastapi import APIRouter
from sqlalchemy import select
from sqlalchemy.orm import selectinload

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import (
    AuthorizationError,
    ResourceNotFoundError,
)
from app.models.similarity import SimilarityRecord
from app.models.task import TrainingTask
from app.models.upload import ParseResult, Upload
from app.schemas.similarity import (
    SegmentPair,
    SimilarityDecision,
    SimilarityRecordOut,
)
from app.services.similarity_service import find_similar_segments

router = APIRouter(prefix="/api/similarity", tags=["similarity"])


@router.get(
    "/task/{task_id}",
    response_model=list[SimilarityRecordOut],
)
async def list_similarity(
    task_id: int,
    db: DbSession,
    current: CurrentUser,
    state: str = "suspect",
) -> list[SimilarityRecordOut]:
    if current.role == "student":
        raise AuthorizationError("仅教师可查看")
    # 跨任务隔离：教师必须是任务教师或 admin
    task = await db.get(TrainingTask, task_id)
    if task is None:
        raise ResourceNotFoundError(f"task {task_id} not found")
    if current.role == "teacher" and task.teacher_id != current.id:
        raise AuthorizationError("无权查看其他教师的任务")

    rows = list(
        (
            await db.execute(
                select(SimilarityRecord).where(
                    SimilarityRecord.task_id == task_id,
                    SimilarityRecord.state == state,
                )
            )
        )
        .scalars()
        .all()
    )
    return [
        SimilarityRecordOut(
            id=r.id,
            task_id=r.task_id,
            upload_a_id=r.upload_a_id,
            upload_b_id=r.upload_b_id,
            hamming_distance=r.hamming_distance,
            cosine_similarity=(
                float(r.cosine_similarity)
                if r.cosine_similarity is not None
                else None
            ),
            state=r.state,
            created_at=r.created_at,
            decided_at=r.decided_at,
        )
        for r in rows
    ]


@router.get("/{record_id}/segments", response_model=list[SegmentPair])
async def get_segments(
    record_id: int, db: DbSession, current: CurrentUser
) -> list[SegmentPair]:
    if current.role == "student":
        raise AuthorizationError("仅教师可查看")
    rec = await db.get(SimilarityRecord, record_id)
    if rec is None:
        raise ResourceNotFoundError(f"record {record_id} not found")

    # 加载两份 parse_result
    pr_a = (
        await db.execute(
            select(ParseResult).where(ParseResult.upload_id == rec.upload_a_id)
        )
    ).scalar_one_or_none()
    pr_b = (
        await db.execute(
            select(ParseResult).where(ParseResult.upload_id == rec.upload_b_id)
        )
    ).scalar_one_or_none()
    if pr_a is None or pr_b is None:
        return []
    segments = find_similar_segments(pr_a.raw_text, pr_b.raw_text)
    return [SegmentPair(**s) for s in segments]  # type: ignore[arg-type]


@router.post("/{record_id}/decision")
async def decide_similarity(
    record_id: int,
    payload: SimilarityDecision,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    if current.role == "student":
        raise AuthorizationError("仅教师可裁决")
    rec = await db.get(SimilarityRecord, record_id)
    if rec is None:
        raise ResourceNotFoundError(f"record {record_id} not found")

    task = await db.get(TrainingTask, rec.task_id)
    if (
        current.role == "teacher"
        and task is not None
        and task.teacher_id != current.id
    ):
        raise AuthorizationError("无权裁决他人任务")

    rec.state = "confirmed" if payload.action == "confirm" else "ignored"
    rec.reviewed_by = current.id
    rec.decided_at = datetime.now(UTC)
    await db.commit()
    return {"id": rec.id, "state": rec.state}
