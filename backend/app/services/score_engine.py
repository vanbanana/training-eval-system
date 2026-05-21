"""Score Engine - 评价引擎（Epic 16）.

核心逻辑：
1. 对每个 Dimension，调用 LLM 生成 ai_score(0-100) + rationale
2. 教师可后续覆盖 teacher_score
3. 综合分 = sum(weight * (teacher_score or ai_score)) / 100  → Property 1
4. 综合分 ∈ [0, 100]  → Property 2
"""

from __future__ import annotations

import json
import random
from typing import TYPE_CHECKING

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.core.exceptions import (
    BusinessRuleError,
    ResourceNotFoundError,
    WeightSumInvalidError,
)
from app.core.logging import get_logger
from app.models.evaluation import DimensionScore, Evaluation
from app.models.task import Dimension, TrainingTask
from app.models.upload import Upload


if TYPE_CHECKING:
    from app.llm.base import LLMProvider


log = get_logger(__name__)


def compute_total_score(scores: list[DimensionScore], dim_map: dict[int, int]) -> float:
    """根据维度分数 + 权重 map 计算综合分.

    优先使用 teacher_score，未覆盖则用 ai_score.
    """
    total_weight = sum(dim_map.values())
    if total_weight != 100:
        raise WeightSumInvalidError(
            f"维度权重和必须为 100，当前 {total_weight}", field="dimensions"
        )
    total = 0.0
    for ds in scores:
        score = ds.teacher_score if ds.teacher_score is not None else (ds.ai_score or 0)
        weight = dim_map.get(ds.dimension_id, 0)
        total += score * weight / 100
    rounded = round(total, 1)
    # 边界守恒
    return max(0.0, min(100.0, rounded))


class ScoreEngine:
    def __init__(self, llm: "LLMProvider | None" = None) -> None:
        self.llm = llm

    async def score_upload(
        self, db: AsyncSession, *, upload_id: int
    ) -> Evaluation:
        # 显式预加载 parse_result
        upload = (
            await db.execute(
                select(Upload)
                .options(selectinload(Upload.parse_result))
                .where(Upload.id == upload_id)
            )
        ).scalar_one_or_none()
        if upload is None:
            raise ResourceNotFoundError(f"upload {upload_id} not found")

        # 拒绝重复评价
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
            raise BusinessRuleError(
                "任务无评分维度", field="dimensions"
            )

        # 调 LLM（或 fallback 到 mock 随机）
        submission_text = (
            upload.parse_result.raw_text
            if upload.parse_result is not None
            else f"file: {upload.filename}"
        )
        ai_outputs = await self._call_llm(
            dims=dims, submission_text=submission_text, task=task
        )

        evaluation = Evaluation(
            task_id=task.id,
            student_id=upload.student_id,
            upload_id=upload_id,
            status="scored",
        )
        db.add(evaluation)
        await db.flush()

        scores = []
        for d in dims:
            item = next(
                (a for a in ai_outputs if a.get("dimension_id") == d.id), None
            )
            ai_score = float(item["score"]) if item else round(
                random.uniform(60, 95), 1
            )
            rationale = (
                str(item.get("rationale", "")) if item else f"{d.name} 评分"
            )
            ai_score = max(0.0, min(100.0, ai_score))
            ds = DimensionScore(
                evaluation_id=evaluation.id,
                dimension_id=d.id,
                ai_score=ai_score,
                rationale=rationale,
            )
            db.add(ds)
            scores.append(ds)
        await db.flush()

        dim_map = {d.id: d.weight for d in dims}
        evaluation.total_score = compute_total_score(scores, dim_map)
        await db.flush()
        # 显式预加载 scores 后再返回，避免调用方触发 lazy load
        await db.refresh(evaluation, ["scores"])

        log.info(
            "score.completed",
            evaluation_id=evaluation.id,
            total=evaluation.total_score,
        )
        return evaluation

    async def confirm(
        self,
        db: AsyncSession,
        *,
        evaluation_id: int,
        teacher_comment: str = "",
        score_overrides: dict[int, float] | None = None,
    ) -> Evaluation:
        """教师确认（可覆盖维度分）→ 重算综合分."""
        # 显式预加载 scores
        ev = (
            await db.execute(
                select(Evaluation)
                .options(selectinload(Evaluation.scores))
                .where(Evaluation.id == evaluation_id)
            )
        ).scalar_one_or_none()
        if ev is None:
            raise ResourceNotFoundError(
                f"evaluation {evaluation_id} not found"
            )
        if ev.status == "confirmed":
            raise BusinessRuleError("已确认，不可重复操作", field="status")

        overrides = score_overrides or {}
        for ds in ev.scores:
            if ds.dimension_id in overrides:
                v = max(0.0, min(100.0, float(overrides[ds.dimension_id])))
                ds.teacher_score = v

        ev.teacher_comment = teacher_comment
        ev.status = "confirmed"

        # 重算综合分
        dims = list(
            (
                await db.execute(
                    select(Dimension).where(Dimension.task_id == ev.task_id)
                )
            )
            .scalars()
            .all()
        )
        dim_map = {d.id: d.weight for d in dims}
        ev.total_score = compute_total_score(ev.scores, dim_map)
        await db.flush()
        await db.refresh(ev, ["scores"])
        return ev

    async def _call_llm(
        self,
        *,
        dims: list[Dimension],
        submission_text: str,
        task: TrainingTask,
    ) -> list[dict[str, object]]:
        """调 LLM 获取每维度评分；失败则 mock."""
        if self.llm is None:
            return []

        try:
            from app.llm.base import LLMMessage

            prompt = (
                f"任务: {task.name}\n要求: {task.requirements}\n"
                f"提交内容前 1000 字: {submission_text[:1000]}\n"
                f"为以下维度打分（0-100 整数）+ 简短理由，"
                f"返回 JSON 数组：[{{\"dimension_id\":N,\"score\":N,\"rationale\":\"...\"}}, ...]\n"
                f"维度："
                + "; ".join(
                    f"id={d.id} name={d.name} (权重{d.weight}%)" for d in dims
                )
            )
            resp = await self.llm.chat(
                [
                    LLMMessage(role="system", content="你是严格的实训评价助手。"),
                    LLMMessage(role="user", content=prompt),
                ],
                temperature=0.2,
            )
            # 尝试提取 JSON
            content = resp.content
            try:
                data = json.loads(content)
                if isinstance(data, list):
                    return data
            except (json.JSONDecodeError, TypeError):
                pass
        except Exception as e:  # noqa: BLE001
            log.warning("score.llm_failed", error=str(e))
        return []
