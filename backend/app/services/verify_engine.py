"""Verify Engine - 智能核查引擎（Epic 15）.

输入：ParseResult.raw_text + 任务要求 (TrainingTask.requirements)
输出：VerifyResult（match_rate, missing_items, logic_issues, overall_confidence）
"""

from __future__ import annotations

import re
from typing import TYPE_CHECKING

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.core.exceptions import ResourceNotFoundError
from app.core.logging import get_logger
from app.models.task import TrainingTask
from app.models.upload import ParseResult, Upload, VerifyResult


if TYPE_CHECKING:
    from app.llm.base import LLMProvider


log = get_logger(__name__)


def extract_checkpoints(requirements: str) -> list[str]:
    """从 task.requirements 文本中提取检查点（每行一个或编号项）."""
    checkpoints: list[str] = []
    for line in requirements.splitlines():
        line = line.strip()
        if not line:
            continue
        # 去除编号前缀（1. 一、 (1)）
        line = re.sub(r"^[\d一二三四五六七八九十（()【\[]+[、.）)]\s*", "", line)
        if line:
            checkpoints.append(line)
    return checkpoints


# 常见停用词（避免 bigram 误匹配）
_STOPWORDS = {
    "完成",
    "提交",
    "包含",
    "提供",
    "撰写",
    "写出",
    "实现",
    "需要",
    "请",
    "应该",
    "且",
    "并",
    "及",
    "和",
    "与",
    "以",
}


def _bigrams(s: str) -> list[str]:
    s = re.sub(r"\s+", "", s)
    return [s[i : i + 2] for i in range(len(s) - 1)]


def keyword_match_rate(text: str, checkpoints: list[str]) -> tuple[float, list[str]]:
    """关键词匹配，返回 (匹配率 0-100, 缺失项列表).

    匹配策略：
    1. 整体 substring 命中视为匹配
    2. 否则按 bigram 滑窗，过滤停用词后命中率 ≥ 40% 视为匹配
    """
    if not checkpoints:
        return 100.0, []
    matched = 0
    missing: list[str] = []
    text_lower = text.lower()
    for cp in checkpoints:
        cp_lower = cp.lower().strip()
        if not cp_lower:
            continue
        # 整体或前缀子串
        if cp_lower in text_lower:
            matched += 1
            continue
        # bigram 召回
        bigrams = [b for b in _bigrams(cp_lower) if b not in _STOPWORDS]
        if not bigrams:
            missing.append(cp)
            continue
        hit = sum(1 for b in bigrams if b in text_lower)
        if hit / len(bigrams) >= 0.4:
            matched += 1
        else:
            missing.append(cp)
    rate = matched / len(checkpoints) * 100 if checkpoints else 100.0
    return rate, missing


class VerifyEngine:
    def __init__(self, llm: "LLMProvider | None" = None) -> None:
        self.llm = llm

    async def run(
        self, db: AsyncSession, *, upload_id: int
    ) -> VerifyResult:
        # 显式预加载 parse_result，避免 async 下隐式 lazy load
        upload = (
            await db.execute(
                select(Upload)
                .options(selectinload(Upload.parse_result))
                .where(Upload.id == upload_id)
            )
        ).scalar_one_or_none()
        if upload is None:
            raise ResourceNotFoundError(f"upload {upload_id} not found")
        parse_result: ParseResult | None = upload.parse_result
        if parse_result is None:
            raise ResourceNotFoundError(
                f"upload {upload_id} 尚未完成解析", field="parse_result"
            )

        task = await db.get(TrainingTask, upload.task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {upload.task_id} not found")

        text = parse_result.raw_text or ""

        checkpoints = extract_checkpoints(task.requirements)
        match_rate, missing = keyword_match_rate(text, checkpoints)

        # 简化：logic_issues 留给 LLM Skill 在生产中检测
        logic_issues: list[dict[str, object]] = []

        # 综合置信度：基于 match_rate 与文本长度的简单启发
        text_score = min(100, len(text) // 10) if text else 0
        overall_confidence = int((match_rate * 0.7 + text_score * 0.3))

        verify_result = VerifyResult(
            upload_id=upload_id,
            match_rate=round(match_rate, 2),
            checkpoints=[
                {"text": cp, "matched": cp not in missing} for cp in checkpoints
            ],
            missing_items=missing,
            logic_issues=logic_issues,
            overall_confidence=overall_confidence,
        )
        db.add(verify_result)
        await db.flush()
        await db.refresh(verify_result)
        log.info(
            "verify.completed",
            upload_id=upload_id,
            match_rate=match_rate,
            missing_count=len(missing),
        )
        return verify_result
