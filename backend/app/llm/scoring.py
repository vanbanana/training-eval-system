"""LLM 评分 Skill - 按维度对学生提交进行评分."""

from __future__ import annotations

import random

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.logging import get_logger
from app.llm.client import call_llm_json, get_active_config
from app.models.task import Dimension

log = get_logger(__name__)

SCORING_SYSTEM_PROMPT = """你是一个专业的实训评价助手。你需要根据学生提交的实训成果，按照给定的评价维度进行打分。

要求：
1. 每个维度给出 0-100 的整数分数
2. 每个维度给出简短的评分依据（50-100字）
3. 评分要客观、公正，基于实际内容质量

请以 JSON 格式返回，格式如下：
{
  "scores": [
    {"dimension_id": 1, "score": 85, "rationale": "代码结构清晰..."},
    ...
  ]
}
"""


async def score_submission(
    db: AsyncSession,
    *,
    dimensions: list[Dimension],
    submission_text: str,
    task_description: str,
) -> list[dict]:
    """对提交内容按维度评分。有 LLM 配置时真实调用，否则 mock."""
    config = await get_active_config(db)

    if not config:
        log.info("llm.scoring.mock", reason="no_config")
        return [
            {"dimension_id": d.id, "score": round(random.uniform(60, 95), 1), "rationale": f"（mock）{d.name} 维度表现良好"}
            for d in dimensions
        ]

    # 构建 prompt
    dim_desc = "\n".join(f"- 维度 {d.id}「{d.name}」（权重 {d.weight}%）：{d.description}" for d in dimensions)
    user_prompt = f"""## 任务要求
{task_description}

## 评价维度
{dim_desc}

## 学生提交内容
{submission_text[:3000]}

请按上述维度逐一评分，返回 JSON。"""

    try:
        result = await call_llm_json(db, system_prompt=SCORING_SYSTEM_PROMPT, user_prompt=user_prompt)
        if "scores" in result:
            return result["scores"]
        else:
            log.warning("llm.scoring.bad_format", result=result)
            # fallback mock
            return [
                {"dimension_id": d.id, "score": round(random.uniform(60, 95), 1), "rationale": f"（LLM 返回格式异常，使用 mock）{d.name}"}
                for d in dimensions
            ]
    except Exception as e:
        log.error("llm.scoring.failed", error=str(e))
        return [
            {"dimension_id": d.id, "score": round(random.uniform(60, 95), 1), "rationale": f"（LLM 调用失败：{e}，使用 mock）"}
            for d in dimensions
        ]
