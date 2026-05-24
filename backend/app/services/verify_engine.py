"""Verify Engine - 智能核查引擎.

输入：ParseResult.raw_text + 任务要求 (TrainingTask.requirements)
输出：VerifyResult（match_rate, checkpoints, missing_items, logic_issues, overall_confidence）

两阶段核查：
1. 关键词/bigram 匹配（快速，无需 LLM）
2. LLM 深度核查（覆盖度 + 逻辑漏洞检测）
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
    """智能核查引擎.

    两阶段：
    1. 快速关键词匹配（无 LLM 依赖）
    2. LLM 深度核查（有 LLM 时执行覆盖度检查 + 逻辑漏洞检测）
    """

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

        # ===== 阶段 1: 快速关键词匹配 =====
        checkpoints = extract_checkpoints(task.requirements)
        match_rate, missing = keyword_match_rate(text, checkpoints)

        # ===== 阶段 2: LLM 深度核查（可选）=====
        logic_issues: list[dict[str, object]] = []
        llm_checkpoints: list[dict[str, object]] | None = None

        if self.llm is not None and text:
            llm_checkpoints, logic_issues = await self._llm_verify(
                task_requirements=task.requirements,
                parse_text=text,
                structured_content=parse_result.structured_content,
            )
            # 如果 LLM 返回了覆盖度检查结果，用 LLM 结果替代关键词匹配
            if llm_checkpoints:
                matched_count = sum(
                    1 for cp in llm_checkpoints if cp.get("matched")
                )
                total = len(llm_checkpoints)
                if total > 0:
                    match_rate = matched_count / total * 100
                    missing = [
                        cp.get("requirement", "")
                        for cp in llm_checkpoints
                        if not cp.get("matched")
                    ]

        # 综合置信度
        text_score = min(100, len(text) // 10) if text else 0
        overall_confidence = int((match_rate * 0.7 + text_score * 0.3))

        # 构建 checkpoints 输出
        final_checkpoints: list[dict[str, object]]
        if llm_checkpoints:
            final_checkpoints = llm_checkpoints
        else:
            final_checkpoints = [
                {"requirement": cp, "matched": cp not in missing, "confidence": 70}
                for cp in checkpoints
            ]

        # 删除旧的 VerifyResult（重新核查场景）
        if upload.verify_result is not None:
            await db.delete(upload.verify_result)
            await db.flush()

        verify_result = VerifyResult(
            upload_id=upload_id,
            match_rate=round(match_rate, 2),
            checkpoints=final_checkpoints,
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
            logic_issues_count=len(logic_issues),
            used_llm=self.llm is not None,
        )
        return verify_result

    async def _llm_verify(
        self,
        *,
        task_requirements: str,
        parse_text: str,
        structured_content: dict | None,
    ) -> tuple[list[dict[str, object]], list[dict[str, object]]]:
        """调用 LLM Skills 进行深度核查.

        返回 (checkpoints, logic_issues)。
        任一 Skill 失败不阻塞，降级为空结果。
        """
        checkpoints: list[dict[str, object]] = []
        logic_issues: list[dict[str, object]] = []

        if self.llm is None:
            return checkpoints, logic_issues

        # 提取摘要和要点
        summary = ""
        key_points: list[str] = []
        if structured_content:
            summary = structured_content.get("llm_summary", "") or ""
            key_points = structured_content.get("llm_key_topics", []) or []
        if not summary:
            summary = parse_text[:2000]

        # 覆盖度检查
        try:
            from app.llm.skills.verify.coverage_check import (
                CoverageCheckSkill,
                CoverageInput,
            )

            skill = CoverageCheckSkill()
            input_data = CoverageInput(
                task_requirements=task_requirements[:3000],
                parse_summary=summary[:2000],
                parse_key_points=key_points[:20],
            )
            output = await skill.execute(input_data, self.llm)
            checkpoints = [cp.model_dump() for cp in output.checkpoints]
        except Exception as e:  # noqa: BLE001
            log.warning("verify.llm_coverage_failed", error=str(e))

        # 逻辑漏洞检测
        try:
            from app.llm.skills.verify.logic_audit import (
                LogicAuditInput,
                LogicAuditSkill,
            )

            skill2 = LogicAuditSkill()
            input_data2 = LogicAuditInput(
                task_requirements=task_requirements[:3000],
                parse_summary=summary[:2000],
                parse_key_points=key_points[:20],
            )
            output2 = await skill2.execute(input_data2, self.llm)
            logic_issues = [issue.model_dump() for issue in output2.issues]
        except Exception as e:  # noqa: BLE001
            log.warning("verify.llm_logic_audit_failed", error=str(e))

        return checkpoints, logic_issues
