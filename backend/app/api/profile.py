"""学生能力画像 + 教学画像路由."""

from __future__ import annotations

from fastapi import APIRouter
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.models.evaluation import Evaluation
from app.models.task import Dimension

router = APIRouter(prefix="/api/profile", tags=["profile"])


@router.get("/student/{student_id}")
async def student_profile(student_id: int, db: DbSession, current: CurrentUser) -> dict[str, object]:
    """学生薄弱点画像：各维度平均分 + 薄弱点."""
    # 如果是学生只能看自己
    if current.role == "student" and current.id != student_id:
        from app.core.exceptions import AuthorizationError
        raise AuthorizationError("无权查看他人画像")

    evals = (await db.execute(select(Evaluation).where(Evaluation.student_id == student_id, Evaluation.status.in_(["scored", "confirmed"])))).scalars().all()
    if not evals:
        return {"student_id": student_id, "eval_count": 0, "dimensions": [], "weaknesses": [], "message": "评价数据不足，至少需要 1 次评价"}

    # 聚合各维度分数
    dim_scores: dict[int, list[float]] = {}
    for ev in evals:
        for s in ev.scores:
            score = s.teacher_score if s.teacher_score is not None else s.ai_score
            if score is not None:
                dim_scores.setdefault(s.dimension_id, []).append(score)

    # 获取维度名称
    dim_ids = list(dim_scores.keys())
    dims = (await db.execute(select(Dimension).where(Dimension.id.in_(dim_ids)))).scalars().all()
    dim_map = {d.id: d.name for d in dims}

    dimensions = []
    for did, scores in dim_scores.items():
        avg = round(sum(scores) / len(scores), 1)
        dimensions.append({"dimension_id": did, "name": dim_map.get(did, f"维度{did}"), "avg_score": avg, "count": len(scores)})

    dimensions.sort(key=lambda x: x["avg_score"])
    weaknesses = [d for d in dimensions if d["avg_score"] < 75]

    return {
        "student_id": student_id,
        "eval_count": len(evals),
        "total_avg": round(sum(e.total_score for e in evals if e.total_score) / len(evals), 1),
        "dimensions": dimensions,
        "weaknesses": weaknesses[:5],
    }


@router.get("/teaching")
async def teaching_profile(db: DbSession, current: CurrentUser) -> dict[str, object]:
    """教学画像（简化版：全局统计）."""
    evals = (await db.execute(select(Evaluation).where(Evaluation.status.in_(["scored", "confirmed"])))).scalars().all()
    if not evals:
        return {"eval_count": 0, "avg_score": 0, "message": "暂无评价数据"}

    avg = round(sum(e.total_score for e in evals if e.total_score) / len(evals), 1)
    confirmed = len([e for e in evals if e.status == "confirmed"])
    return {
        "eval_count": len(evals),
        "avg_score": avg,
        "confirmed_count": confirmed,
        "adoption_rate": round(confirmed / len(evals) * 100, 1) if evals else 0,
    }
