"""任务编辑路由（补全 Epic 5）."""

from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter
from pydantic import BaseModel

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError, BusinessRuleError, ResourceNotFoundError
from app.models.task import TrainingTask

router = APIRouter(prefix="/api/tasks", tags=["task-edit"])


class UpdateTaskRequest(BaseModel):
    description: str | None = None
    requirements: str | None = None
    deadline: datetime | None = None


@router.patch("/{task_id}")
async def update_task(task_id: int, payload: UpdateTaskRequest, db: DbSession, current: CurrentUser) -> dict[str, object]:
    """编辑任务（published 状态仅可改 description/requirements/deadline）."""
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可操作")
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise ResourceNotFoundError("task not found")
    if task.teacher_id != current.id and current.role != "admin":
        raise AuthorizationError("无权编辑他人任务")
    if task.status == "closed":
        raise BusinessRuleError("已关闭任务不可编辑", field="status")

    if payload.description is not None:
        task.description = payload.description
    if payload.requirements is not None:
        task.requirements = payload.requirements
    if payload.deadline is not None:
        task.deadline = payload.deadline

    await db.commit()
    return {"id": task.id, "status": task.status, "updated": True}


@router.delete("/{task_id}", status_code=204)
async def delete_task(task_id: int, db: DbSession, current: CurrentUser) -> None:
    """删除任务（仅 draft 状态可删）."""
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可操作")
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise ResourceNotFoundError("task not found")
    if task.status != "draft":
        raise BusinessRuleError("仅草稿状态可删除", field="status")
    await db.delete(task)
    await db.commit()
