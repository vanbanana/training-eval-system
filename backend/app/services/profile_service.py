"""ProfileService - Epic 18.3."""

from __future__ import annotations

from collections import defaultdict
from datetime import UTC, datetime
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.core.exceptions import BusinessRuleError, ResourceNotFoundError
from app.core.logging import get_logger
from app.llm.base import LLMProvider
from app.llm.skills.profile import (
    AdviceInput,
    LearningAdviceSkill,
    WeaknessAnalyzeSkill,
    WeaknessInput,
)
from app.llm.skills.profile.weakness_analyze import DimensionStat
from app.models.evaluation import Evaluation
from app.models.profile import StudentProfile
from app.models.task import Dimension


log = get_logger(__name__)

INSUFFICIENT_DATA_ERROR_CODE = "INSUFFICIENT_DATA"
MIN_EVALUATIONS_REQUIRED = 3


class InsufficientDataError(BusinessRuleError):
    error_code = INSUFFICIENT_DATA_ERROR_CODE


class ProfileService:
    def __init__(self, llm: LLMProvider | None = None) -> None:
        self.llm = llm
        self.weak_skill = WeaknessAnalyzeSkill()
        self.advice_skill = LearningAdviceSkill()

    async def compute_student_profile(
        self, db: AsyncSession, *, student_id: int
    ) -> StudentProfile:
        # 加载学生评价（含 scores）
        evals = list(
            (
                await db.execute(
                    select(Evaluation)
                    .options(selectinload(Evaluation.scores))
                    .where(Evaluation.student_id == student_id)
                    .order_by(Evaluation.created_at)
                )
            )
            .scalars()
            .all()
        )
        if len(evals) < MIN_EVALUATIONS_REQUIRED:
            raise InsufficientDataError(
                f"评价数 {len(evals)} 不足 {MIN_EVALUATIONS_REQUIRED} 次",
                field="evaluation_count",
            )

        # 维度统计：按 dimension name 聚合
        dim_id_to_name: dict[int, str] = {}
        # 加载所有用到的 dimension 名
        used_dim_ids = {s.dimension_id for ev in evals for s in ev.scores}
        if used_dim_ids:
            dims = list(
                (
                    await db.execute(
                        select(Dimension).where(Dimension.id.in_(used_dim_ids))
                    )
                )
                .scalars()
                .all()
            )
            dim_id_to_name = {d.id: d.name for d in dims}

        agg: dict[str, list[float]] = defaultdict(list)
        agg_eval: dict[str, list[int]] = defaultdict(list)
        for ev in evals:
            for s in ev.scores:
                name = dim_id_to_name.get(s.dimension_id, f"维度{s.dimension_id}")
                point = s.teacher_score if s.teacher_score is not None else s.ai_score
                if point is not None:
                    agg[name].append(point)
                    agg_eval[name].append(ev.id)

        radar = {
            name: round(sum(vals) / len(vals), 1) for name, vals in agg.items() if vals
        }
        score_trend = [
            {
                "evaluation_id": ev.id,
                "total_score": ev.total_score,
                "created_at": ev.created_at.isoformat(),
            }
            for ev in evals
            if ev.total_score is not None
        ]

        # 调 LLM Skill
        weaknesses_data: list[dict[str, Any]] = []
        suggestions_data: list[dict[str, Any]] = []
        if self.llm is not None and agg:
            try:
                stats = [
                    DimensionStat(
                        name=name,
                        avg_score=sum(vs) / len(vs),
                        min_score=min(vs),
                        count=len(vs),
                    )
                    for name, vs in agg.items()
                ]
                weak_out = await self.weak_skill.execute(
                    WeaknessInput(dimension_stats=stats), self.llm
                )
                weaknesses_data = [w.model_dump() for w in weak_out.weaknesses]

                if weak_out.weaknesses:
                    advice_out = await self.advice_skill.execute(
                        AdviceInput(weaknesses=weak_out.weaknesses), self.llm
                    )
                    suggestions_data = [s.model_dump() for s in advice_out.suggestions]
            except Exception as e:  # noqa: BLE001
                log.warning("profile.llm_failed", error=str(e))

        # UPSERT
        existing = (
            await db.execute(
                select(StudentProfile).where(StudentProfile.student_id == student_id)
            )
        ).scalar_one_or_none()

        if existing is None:
            profile = StudentProfile(
                student_id=student_id,
                radar_data=radar,
                weakness_list=weaknesses_data,
                suggestions=suggestions_data,
                score_trend=score_trend,
                source_evaluation_count=len(evals),
                computed_at=datetime.now(UTC),
            )
            db.add(profile)
        else:
            profile = existing
            profile.radar_data = radar
            profile.weakness_list = weaknesses_data
            profile.suggestions = suggestions_data
            profile.score_trend = score_trend
            profile.source_evaluation_count = len(evals)
            profile.computed_at = datetime.now(UTC)
        await db.flush()
        await db.refresh(profile)
        log.info(
            "profile.computed",
            student_id=student_id,
            weakness_count=len(weaknesses_data),
        )
        return profile

    async def get_profile(
        self, db: AsyncSession, *, student_id: int
    ) -> StudentProfile | None:
        return (
            await db.execute(
                select(StudentProfile).where(StudentProfile.student_id == student_id)
            )
        ).scalar_one_or_none()


# ============== Epic 19 教学画像 ==============


async def aggregate_course_metrics(
    db: AsyncSession, *, course_id: int
) -> dict[str, Any]:
    """聚合课程级指标（替代 PG 物化视图，跨 SQLite/PG 通用）."""
    from app.models.task import Dimension, TrainingTask

    # 该课程下所有 task
    tasks = list(
        (
            await db.execute(
                select(TrainingTask).where(TrainingTask.course_id == course_id)
            )
        )
        .scalars()
        .all()
    )
    task_ids = [t.id for t in tasks]
    if not task_ids:
        return {
            "total_evaluations": 0,
            "total_students": 0,
            "avg_score": 0.0,
            "dimension_distributions": [],
        }

    evals = list(
        (
            await db.execute(
                select(Evaluation)
                .options(selectinload(Evaluation.scores))
                .where(Evaluation.task_id.in_(task_ids))
            )
        )
        .scalars()
        .all()
    )
    total = len(evals)
    student_set = {ev.student_id for ev in evals}
    scores_with_total = [ev.total_score for ev in evals if ev.total_score is not None]
    avg_score = (
        sum(scores_with_total) / len(scores_with_total)
        if scores_with_total
        else 0.0
    )

    # 维度分布
    dim_ids = {s.dimension_id for ev in evals for s in ev.scores}
    dim_name_map: dict[int, str] = {}
    if dim_ids:
        dim_rows = list(
            (
                await db.execute(
                    select(Dimension).where(Dimension.id.in_(dim_ids))
                )
            )
            .scalars()
            .all()
        )
        dim_name_map = {d.id: d.name for d in dim_rows}

    by_dim: dict[str, list[float]] = defaultdict(list)
    for ev in evals:
        for s in ev.scores:
            name = dim_name_map.get(s.dimension_id, f"维度{s.dimension_id}")
            point = s.teacher_score if s.teacher_score is not None else s.ai_score
            if point is not None:
                by_dim[name].append(point)

    distributions = [
        {
            "name": name,
            "avg": round(sum(vs) / len(vs), 1),
            "min": min(vs),
            "max": max(vs),
            "count": len(vs),
            "low_ratio": round(
                sum(1 for v in vs if v < 60) / len(vs), 3
            ),
        }
        for name, vs in by_dim.items()
    ]

    return {
        "total_evaluations": total,
        "total_students": len(student_set),
        "avg_score": round(avg_score, 1),
        "dimension_distributions": distributions,
    }
