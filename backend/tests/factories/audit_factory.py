"""AuditLogFactory - Epic 23.1."""

from __future__ import annotations

import random
import uuid
from datetime import UTC, datetime, timedelta
from typing import Any

from app.models.audit import AuditLog
from sqlalchemy.ext.asyncio import AsyncSession

COMMON_ACTIONS: tuple[tuple[str, str], ...] = (
    ("auth.login", "success"),
    ("auth.logout", "success"),
    ("auth.login.failed", "failed"),
    ("upload.created", "success"),
    ("upload.parsed", "success"),
    ("evaluation.auto_scored", "success"),
    ("evaluation.confirmed", "success"),
    ("evaluation.rejected", "success"),
    ("task.created", "success"),
    ("task.published", "success"),
    ("notification.read", "success"),
    ("llm.call", "success"),
    ("llm.call.timeout", "failed"),
    ("user.created", "success"),
    ("course.archived", "success"),
)


class AuditLogFactory:
    """生成单条审计日志，可指定 occurred_at 模拟时间分布."""

    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        user_id: int | None = None,
        username: str = "",
        role: str = "",
        action: str | None = None,
        result: str | None = None,
        target_type: str = "",
        target_id: str | int = "",
        client_ip: str = "127.0.0.1",
        occurred_at: datetime | None = None,
        detail: str = "",
        payload: dict[str, Any] | None = None,
        rng: random.Random | None = None,
    ) -> AuditLog:
        rnd = rng or random
        if action is None or result is None:
            action, result = rnd.choice(COMMON_ACTIONS)
        row = AuditLog(
            user_id=user_id,
            username=username,
            role=role,
            action=action,
            target_type=target_type,
            target_id=str(target_id) if target_id is not None else "",
            target=f"{target_type}:{target_id}" if target_type else "",
            result=result,
            detail=detail or f"action={action}",
            payload=payload,
            client_ip=client_ip,
            ip=client_ip,
            user_agent="seed/1.0",
            trace_id=uuid.uuid4().hex,
        )
        session.add(row)
        await session.flush()
        # 手动覆盖 occurred_at（server_default=now() 忽略）
        if occurred_at is not None:
            row.occurred_at = occurred_at
            row.created_at = occurred_at
            await session.flush()
        return row

    @classmethod
    async def burst(
        cls,
        session: AsyncSession,
        *,
        users: list[tuple[int, str, str]],
        days: int = 7,
        per_day_per_user: int = 4,
        rng: random.Random | None = None,
    ) -> int:
        """为 (user_id, username, role) 列表生成最近 N 天的日志."""
        rnd = rng or random
        now = datetime.now(UTC)
        count = 0
        for d_offset in range(days):
            day = now - timedelta(days=d_offset)
            for uid, uname, role in users:
                for _ in range(per_day_per_user):
                    occurred = day - timedelta(
                        hours=rnd.randint(0, 23),
                        minutes=rnd.randint(0, 59),
                    )
                    await cls.create_async(
                        session,
                        user_id=uid,
                        username=uname,
                        role=role,
                        occurred_at=occurred,
                        rng=rnd,
                    )
                    count += 1
        return count
