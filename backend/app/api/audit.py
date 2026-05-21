"""审计日志路由."""

from __future__ import annotations

from fastapi import APIRouter, Query
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError
from app.models.audit import AuditLog

router = APIRouter(prefix="/api/audit", tags=["audit"])


@router.get("")
async def list_audit_logs(
    db: DbSession,
    current: CurrentUser,
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
    action: str | None = None,
    username: str | None = None,
) -> dict[str, object]:
    if current.role != "admin":
        raise AuthorizationError("仅管理员可查看审计日志")
    stmt = select(AuditLog).order_by(AuditLog.created_at.desc())
    if action:
        stmt = stmt.where(AuditLog.action == action)
    if username:
        stmt = stmt.where(AuditLog.username.contains(username))
    total_stmt = select(AuditLog)
    if action:
        total_stmt = total_stmt.where(AuditLog.action == action)
    total = len((await db.execute(total_stmt)).scalars().all())
    logs = (await db.execute(stmt.offset((page - 1) * page_size).limit(page_size))).scalars().all()
    return {
        "total": total,
        "page": page,
        "page_size": page_size,
        "items": [
            {
                "id": log.id,
                "user_id": log.user_id,
                "username": log.username,
                "role": log.role,
                "action": log.action,
                "target": log.target,
                "detail": log.detail,
                "ip": log.ip,
                "created_at": log.created_at.isoformat(),
            }
            for log in logs
        ],
    }



# ============== Epic 23.5 admin 标准查询/导出 ==============


@router.get("/logs")
async def admin_query_logs(
    db: DbSession,
    current: CurrentUser,
    user_id: int | None = None,
    action: str | None = None,
    ip: str | None = None,
    offset: int = 0,
    limit: int = 100,
) -> dict[str, object]:
    if current.role != "admin":
        raise AuthorizationError("仅管理员可查询")
    from app.services.audit_service import AuditService

    svc = AuditService()
    items = await svc.list_logs(
        db, user_id=user_id, action=action, ip=ip, offset=offset, limit=limit
    )
    return {
        "items": [
            {
                "id": x.id,
                "occurred_at": x.occurred_at.isoformat(),
                "user_id": x.user_id,
                "username": x.username,
                "role": x.role,
                "action": x.action,
                "target_type": x.target_type,
                "target_id": x.target_id,
                "result": x.result,
                "client_ip": x.client_ip,
                "trace_id": x.trace_id,
            }
            for x in items
        ]
    }


@router.get("/export")
async def admin_export_logs(
    db: DbSession, current: CurrentUser
):
    if current.role != "admin":
        raise AuthorizationError("仅管理员可导出")
    import csv
    import io

    from fastapi.responses import StreamingResponse

    from app.services.audit_service import AuditService

    svc = AuditService()
    items = await svc.list_logs(db, limit=10000)
    out = io.StringIO()
    writer = csv.writer(out)
    writer.writerow(
        [
            "occurred_at",
            "user_id",
            "username",
            "role",
            "action",
            "target_type",
            "target_id",
            "result",
            "client_ip",
        ]
    )
    for x in items:
        writer.writerow(
            [
                x.occurred_at.isoformat(),
                x.user_id or "",
                x.username,
                x.role,
                x.action,
                x.target_type,
                x.target_id,
                x.result,
                x.client_ip,
            ]
        )
    out.seek(0)
    return StreamingResponse(
        iter([out.getvalue()]),
        media_type="text/csv",
        headers={
            "Content-Disposition": 'attachment; filename="audit_logs.csv"'
        },
    )
