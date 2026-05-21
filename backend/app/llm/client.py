"""LLM 客户端 - OpenAI 兼容协议调用."""

from __future__ import annotations

import json

from openai import AsyncOpenAI
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.logging import get_logger
from app.models.llm_config import LlmConfig

log = get_logger(__name__)


async def get_active_config(db: AsyncSession) -> LlmConfig | None:
    """获取当前活跃的 LLM 配置."""
    result = await db.execute(select(LlmConfig).where(LlmConfig.is_active == True).limit(1))  # noqa: E712
    return result.scalar_one_or_none()


async def call_llm(
    db: AsyncSession,
    *,
    system_prompt: str,
    user_prompt: str,
    temperature: float = 0.2,
    max_tokens: int = 2000,
) -> str:
    """调用 LLM API，返回文本响应."""
    config = await get_active_config(db)
    if not config:
        raise RuntimeError("未配置 LLM 服务，请先在管理后台配置")

    client = AsyncOpenAI(
        api_key=config.api_key_encrypted,  # dev 阶段明文存储
        base_url=config.base_url,
        timeout=60.0,
    )

    log.info("llm.call.start", provider=config.provider, model=config.chat_model)

    response = await client.chat.completions.create(
        model=config.chat_model,
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
        temperature=temperature,
        max_tokens=max_tokens,
    )

    content = response.choices[0].message.content or ""
    log.info("llm.call.done", tokens=response.usage.total_tokens if response.usage else 0)
    return content


async def call_llm_json(
    db: AsyncSession,
    *,
    system_prompt: str,
    user_prompt: str,
    temperature: float = 0.2,
) -> dict:
    """调用 LLM 并解析 JSON 响应."""
    raw = await call_llm(db, system_prompt=system_prompt, user_prompt=user_prompt, temperature=temperature)
    # 尝试提取 JSON
    try:
        # 处理 markdown code block
        if "```json" in raw:
            raw = raw.split("```json")[1].split("```")[0]
        elif "```" in raw:
            raw = raw.split("```")[1].split("```")[0]
        return json.loads(raw.strip())
    except (json.JSONDecodeError, IndexError) as e:
        log.warning("llm.json_parse_failed", raw_preview=raw[:200], error=str(e))
        return {"error": "JSON 解析失败", "raw": raw[:500]}
