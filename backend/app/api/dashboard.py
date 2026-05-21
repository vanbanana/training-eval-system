"""仪表盘路由 - Epic 24.2."""

from __future__ import annotations

from fastapi import APIRouter

from app.api.deps import CurrentUser, DbSession
from app.services.dashboard_service import DashboardService

router = APIRouter(prefix="/api/dashboard", tags=["dashboard"])


@router.get("")
async def get_dashboard(
    db: DbSession, current: CurrentUser
) -> dict[str, object]:
    svc = DashboardService()
    return await svc.get_dashboard(db, user=current)
