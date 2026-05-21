"""EvaluationService - 评价编排（Epic 16.4 - 16.7）.

依赖：
- DimensionScoreSkill（自动客观评分）
- compute_final_score（综合分计算）
- AuthorizationError / BusinessRuleError（权限与规则）
"""

from __future__ import annotations

import asyncio
from decimal import Decimal
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
from app.llm.base import LLMProvider
from app.llm.skills.base import SkillOutputError
from app.llm.skills.score import (
    DimensionInfo,
    DimensionScoreInput,
    DimensionScoreSkill,
)
from app.models.evaluation import (
    DimensionScore,
    Evaluation,
    EvaluationHistory,
)
from app.models.task import Dimension, TrainingTask
from app.models.upload import Upload
from app.models.user import User
from app.services.scoring import DimensionScoreData, compute_final_score


log = get_logger(__name__)


SUBJECTIVE_INCOMPLETE_ERROR_CODE = "SUBJECTIVE_INCOMPLETE"


class SubjectiveIncompleteError(BusinessRuleError):
    error_code = SUBJECTIVE_INCOMPLETE_ERROR_CODE


class EvaluationService:
    """评价编排服务."""

    def __init__(self, llm: LLMProvider | None = None, *, alpha: float = 0.6) -> None:
        self.llm = llm
        self.alpha = alpha
        self.skill = DimensionScoreSkill()

    # =============== 自动评分（16.4）===============

    async def auto_score(
        self, db: AsyncSession, *, upload_id: int, parse_summary: str = ""
    ) -> Evaluation:
        upload = await db.get(Upload, upload_id)
        if upload is None:
            raise ResourceNotFoundError(f"upload {upload_id} not found")

        existing = (
            await db.execute(
                select(Evaluation).where(Evaluation.upload_id == upload_id)
            )
        ).scalar_one_or_none()
        if existing is not None:
            raise BusinessRuleError(
                "该提交已有评价记录", field="upload_id"
            )

        task = await db.get(TrainingTask, upload.task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {upload.task_id} not found")

        dims = list(
            (
                await db.execute(
                    select(Dimension).where(Dimension.task_id == task.id)
                )
            )
            .scalars()
            .all()
        )
        if not dims:
            raise BusinessRuleError("任务无评分维度", field="dimensions")

        # 并行调 Skill 给每个维度评分
        results = await asyncio.gather(
            *[self._call_skill(d, task.requirements, parse_summary) for d in dims],
            return_exceptions=True,
        )

        evaluation = Evaluation(
            task_id=task.id,
            student_id=upload.student_id,
            upload_id=upload_id,
            status="auto_scored",
        )
        db.add(evaluation)
        await db.flush()

        all_failed = True
        for d, r in zip(dims, results, strict=True):
            if isinstance(r, Exception):
                ai_score: float | None = None
                rationale = f"LLM 失败：{type(r).__name__}"
            else:
                ai_score = float(r["score"])
                rationale = str(r["rationale"])
                all_failed = False
            db.add(
                DimensionScore(
                    evaluation_id=evaluation.id,
                    dimension_id=d.id,
                    ai_score=ai_score,
                    rationale=rationale,
                )
            )
        await db.flush()

        # 计算综合分
        await db.refresh(evaluation, ["scores"])
        if all_failed:
            evaluation.status = "auto_failed"
            evaluation.total_score = 0.0
        else:
            score_data = [
                DimensionScoreData(
                    weight=d.weight,
                    objective_score=ds.ai_score,
                    subjective_score=ds.teacher_score,
                )
                for d, ds in zip(
                    dims,
                    sorted(evaluation.scores, key=lambda s: s.dimension_id),
                    strict=True,
                )
            ]
            try:
                evaluation.total_score = float(
                    compute_final_score(score_data, self.alpha)
                )
            except ValueError as e:
                log.warning("scoring.fallback_zero", error=str(e))
                evaluation.total_score = 0.0

        # 写历史
        db.add(
            EvaluationHistory(
                evaluation_id=evaluation.id,
                operator_id=None,
                action="auto_score",
                before_value=None,
                after_value={
                    "status": evaluation.status,
                    "total_score": evaluation.total_score,
                },
            )
        )
        await db.flush()
        await db.refresh(evaluation, ["scores"])
        log.info(
            "evaluation.auto_score.completed",
            evaluation_id=evaluation.id,
            total=evaluation.total_score,
            status=evaluation.status,
        )
        return evaluation

    async def _call_skill(
        self, dim: Dimension, requirements: str, parse_summary: str
    ) -> dict[str, Any]:
        if self.llm is None:
            raise SkillOutputError("LLM 未配置")
        out = await self.skill.execute(
            DimensionScoreInput(
                task_requirements=requirements,
                dimension=DimensionInfo(name=dim.name, description=dim.description),
                parse_summary=parse_summary,
            ),
            self.llm,
        )
        return {"score": out.score, "rationale": out.rationale}

    # =============== 教师手动评分（16.5）===============

    async def update_dimension_subjective(
        self,
        db: AsyncSession,
        *,
        evaluation_id: int,
        dimension_id: int,
        subj_score: float,
        comment: str = "",
        operator: User,
    ) -> Evaluation:
        if subj_score < 0 or subj_score > 100:
            raise BusinessRuleError(
                f"score {subj_score} 越界", field="subj_score"
            )
        if len(comment) > 500:
            raise BusinessRuleError("comment 不可超过 500 字", field="comment")

        ev = (
            await db.execute(
                select(Evaluation)
                .options(selectinload(Evaluation.scores))
                .where(Evaluation.id == evaluation_id)
            )
        ).scalar_one_or_none()
        if ev is None:
            raise ResourceNotFoundError(f"evaluation {evaluation_id} not found")

        # 仅任务教师可改
        task = await db.get(TrainingTask, ev.task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {ev.task_id} not found")
        if operator.role != "admin" and operator.id != task.teacher_id:
            raise AuthorizationError("仅任务教师可修改")

        target = next(
            (s for s in ev.scores if s.dimension_id == dimension_id), None
        )
        if target is None:
            raise ResourceNotFoundError(
                f"dimension {dimension_id} 无评分记录"
            )

        before = {
            "subj_score": target.teacher_score,
            "rationale": target.rationale,
        }
        target.teacher_score = subj_score
        if comment:
            target.rationale = comment

        # 重算
        dims = list(
            (
                await db.execute(
                    select(Dimension)
                    .where(Dimension.task_id == ev.task_id)
                    .order_by(Dimension.id)
                )
            )
            .scalars()
            .all()
        )
        score_data = [
            DimensionScoreData(
                weight=d.weight,
                objective_score=next(
                    (s.ai_score for s in ev.scores if s.dimension_id == d.id),
                    None,
                ),
                subjective_score=next(
                    (
                        s.teacher_score
                        for s in ev.scores
                        if s.dimension_id == d.id
                    ),
                    None,
                ),
            )
            for d in dims
        ]
        ev.total_score = float(compute_final_score(score_data, self.alpha))
        if ev.status in ("auto_scored", "auto_failed"):
            ev.status = "reviewed"

        db.add(
            EvaluationHistory(
                evaluation_id=ev.id,
                operator_id=operator.id,
                action="manual_score",
                before_value=before,
                after_value={
                    "subj_score": subj_score,
                    "rationale": target.rationale,
                },
            )
        )
        await db.flush()
        await db.refresh(ev, ["scores"])
        return ev

    # =============== 批量审批（16.6）===============

    async def bulk_action(
        self,
        db: AsyncSession,
        *,
        evaluation_ids: list[int],
        action: str,
        operator: User,
        reason: str = "",
    ) -> dict[str, Any]:
        if action not in ("confirm", "reject"):
            raise BusinessRuleError(f"action {action} 无效", field="action")
        if action == "reject" and not reason:
            raise BusinessRuleError("reject 必须提供 reason", field="reason")

        if not evaluation_ids:
            return {"affected": 0}

        evs = list(
            (
                await db.execute(
                    select(Evaluation)
                    .options(selectinload(Evaluation.scores))
                    .where(Evaluation.id.in_(evaluation_ids))
                )
            )
            .scalars()
            .all()
        )

        # confirm 前先检查所有维度都有 subj_score
        if action == "confirm":
            for ev in evs:
                if any(s.teacher_score is None for s in ev.scores):
                    raise SubjectiveIncompleteError(
                        f"evaluation {ev.id} 仍有维度缺主观分",
                        field="scores",
                    )

        for ev in evs:
            before = {"status": ev.status, "comment": ev.teacher_comment}
            if action == "confirm":
                ev.status = "finalized"
            else:
                ev.status = "rejected"
                ev.teacher_comment = reason
            db.add(
                EvaluationHistory(
                    evaluation_id=ev.id,
                    operator_id=operator.id,
                    action=action,
                    before_value=before,
                    after_value={
                        "status": ev.status,
                        "comment": ev.teacher_comment,
                    },
                )
            )
        await db.flush()
        log.info(
            "evaluation.bulk_action.completed",
            count=len(evs),
            action=action,
            operator_id=operator.id,
        )
        return {"affected": len(evs), "action": action}

    # =============== 权重变更触发批量重算（16.7）===============

    async def recalc_for_task(
        self, db: AsyncSession, *, task_id: int, alpha: float | None = None
    ) -> int:
        """alpha 修改后批量重算某 task 下所有 evaluations."""
        if alpha is not None:
            if not 0 <= alpha <= 1:
                raise BusinessRuleError(f"alpha {alpha} 越界", field="alpha")
            self.alpha = alpha
        evs = list(
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
        dims = list(
            (
                await db.execute(
                    select(Dimension)
                    .where(Dimension.task_id == task_id)
                    .order_by(Dimension.id)
                )
            )
            .scalars()
            .all()
        )
        affected = 0
        for ev in evs:
            score_data = [
                DimensionScoreData(
                    weight=d.weight,
                    objective_score=next(
                        (
                            s.ai_score
                            for s in ev.scores
                            if s.dimension_id == d.id
                        ),
                        None,
                    ),
                    subjective_score=next(
                        (
                            s.teacher_score
                            for s in ev.scores
                            if s.dimension_id == d.id
                        ),
                        None,
                    ),
                )
                for d in dims
            ]
            try:
                ev.total_score = float(
                    compute_final_score(score_data, self.alpha)
                )
                affected += 1
            except ValueError:
                continue
        await db.flush()
        return affected
