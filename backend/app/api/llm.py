"""LLM 配置路由."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError
from app.models.llm_config import LlmConfig

router = APIRouter(prefix="/api/llm", tags=["llm"])


def _require_admin(current: object) -> None:
    if getattr(current, "role", None) != "admin":
        raise AuthorizationError("仅管理员可操作")


@router.get("/configs")
async def list_configs(db: DbSession, current: CurrentUser) -> list[dict[str, object]]:
    _require_admin(current)
    configs = (await db.execute(select(LlmConfig).order_by(LlmConfig.id))).scalars().all()
    return [
        {
            "id": c.id,
            "provider": c.provider,
            "base_url": c.base_url,
            "api_key_masked": c.api_key_encrypted[:8] + "****",
            "chat_model": c.chat_model,
            "embed_model": c.embed_model,
            "is_active": c.is_active,
        }
        for c in configs
    ]


class LlmConfigRequest(BaseModel):
    provider: str = Field(..., min_length=1)
    base_url: str = Field(..., min_length=5)
    api_key: str = Field(..., min_length=8)
    chat_model: str = Field(..., min_length=1)
    embed_model: str = ""


@router.post("/configs", status_code=201)
async def create_config(payload: LlmConfigRequest, db: DbSession, current: CurrentUser) -> dict[str, object]:
    _require_admin(current)
    # 如果已有同 provider 的配置则更新，否则新建
    existing = (await db.execute(select(LlmConfig).where(LlmConfig.provider == payload.provider))).scalar_one_or_none()
    if existing:
        existing.base_url = payload.base_url
        if payload.api_key and payload.api_key != "":
            existing.api_key_encrypted = payload.api_key
        existing.chat_model = payload.chat_model
        existing.embed_model = payload.embed_model
        existing.is_active = True
        await db.commit()
        return {"id": existing.id, "provider": existing.provider, "updated": True}
    else:
        config = LlmConfig(
            provider=payload.provider,
            base_url=payload.base_url,
            api_key_encrypted=payload.api_key,
            chat_model=payload.chat_model,
            embed_model=payload.embed_model,
        )
        db.add(config)
        await db.commit()
        await db.refresh(config)
        return {"id": config.id, "provider": config.provider, "updated": False}


@router.post("/test")
async def test_connection(db: DbSession, current: CurrentUser) -> dict[str, object]:
    """测试 LLM 连通性（真实调用）."""
    _require_admin(current)
    import time

    from app.llm.client import get_active_config
    active = await get_active_config(db)
    if not active:
        return {"status": "no_config", "message": "未配置 LLM 服务"}
    try:
        from openai import AsyncOpenAI
        client = AsyncOpenAI(api_key=active.api_key_encrypted, base_url=active.base_url, timeout=15.0)
        start = time.time()
        resp = await client.chat.completions.create(
            model=active.chat_model,
            messages=[{"role": "user", "content": "ping"}],
            max_tokens=5,
        )
        latency = int((time.time() - start) * 1000)
        # 简单消费一次响应内容以验证连通性
        _ = resp.choices[0].message.content if resp.choices else None
        return {"status": "ok", "provider": active.provider, "model": active.chat_model, "latency_ms": latency}
    except Exception as e:
        return {"status": "error", "message": str(e)}
