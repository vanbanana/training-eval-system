"""AuditService - Epic 23.2."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.logging import get_logger
from app.models.audit import AuditLog


log = get_logger(__name__)


class AuditService:
    async def emit(
        self,
        db: AsyncSession,
        *,
        action: str,
        target_type: str = "",
        target_id: str | int = "",
        result: str = "success",
        user_id: int | None = None,
        username: str = "",
        role: str = "",
        detail: str = "",
        payload: dict[str, Any] | None = None,
        client_ip: str = "",
        user_agent: str = "",
        trace_id: str = "",
    ) -> AuditLog:
        try:
            row = AuditLog(
                user_id=user_id,
                username=username,
                role=role,
                action=action,
                target_type=target_type,
                target_id=str(target_id) if target_id is not None else "",
                target=f"{target_type}:{target_id}" if target_type else "",
                result=result,
                detail=detail,
                payload=payload,
                client_ip=client_ip,
                user_agent=user_agent,
                trace_id=trace_id,
                ip=client_ip,
            )
            db.add(row)
            await db.flush()
            return row
        except Exception as e:  # noqa: BLE001
            # emit 失败不阻塞业务
            log.warning("audit.emit_failed", action=action, error=str(e))
            raise

    async def list_logs(
        self,
        db: AsyncSession,
        *,
        from_dt: datetime | None = None,
        to_dt: datetime | None = None,
        user_id: int | None = None,
        action: str | None = None,
        ip: str | None = None,
        offset: int = 0,
        limit: int = 100,
    ) -> list[AuditLog]:
        stmt = select(AuditLog)
        if from_dt:
            stmt = stmt.where(AuditLog.occurred_at >= from_dt)
        if to_dt:
            stmt = stmt.where(AuditLog.occurred_at <= to_dt)
        if user_id is not None:
            stmt = stmt.where(AuditLog.user_id == user_id)
        if action:
            stmt = stmt.where(AuditLog.action == action)
        if ip:
            stmt = stmt.where(AuditLog.client_ip == ip)
        stmt = (
            stmt.order_by(AuditLog.occurred_at.desc())
            .offset(offset)
            .limit(limit)
        )
        return list((await db.execute(stmt)).scalars().all())

    async def detect_suspicious_users(
        self, db: AsyncSession, *, window_minutes: int = 5, threshold: int = 10
    ) -> list[int]:
        """返回 N 分钟内失败次数 ≥ threshold 的 user_id 列表."""
        since = datetime.now(UTC) - timedelta(minutes=window_minutes)
        rows = list(
            (
                await db.execute(
                    select(AuditLog).where(
                        AuditLog.result == "failed",
                        AuditLog.occurred_at >= since,
                    )
                )
            )
            .scalars()
            .all()
        )
        counter: dict[int, int] = {}
        for r in rows:
            if r.user_id is None:
                continue
            counter[r.user_id] = counter.get(r.user_id, 0) + 1
        return [uid for uid, c in counter.items() if c >= threshold]
