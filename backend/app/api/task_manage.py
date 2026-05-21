"""教师任务创建/编辑/状态管理路由."""

from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter
from pydantic import BaseModel, Field

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError, BusinessRuleError, ResourceNotFoundError
from app.models.task import Dimension, TrainingTask

router = APIRouter(prefix="/api/tasks", tags=["task-manage"])


class DimensionInput(BaseModel):
    name: str = Field(..., min_length=1, max_length=64)
    description: str = ""
    weight: int = Field(..., ge=1, le=100)


class CreateTaskRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    description: str = ""
    requirements: str = ""
    course_id: int
    deadline: datetime | None = None
    dimensions: list[DimensionInput] = Field(..., min_length=2, max_length=10)


@router.post("", status_code=201)
async def create_task(payload: CreateTaskRequest, db: DbSession, current: CurrentUser) -> dict[str, object]:
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可创建任务")

    # 校验维度权重和 = 100
    total_weight = sum(d.weight for d in payload.dimensions)
    if total_weight != 100:
        raise BusinessRuleError(f"维度权重和必须为 100，当前为 {total_weight}", field="dimensions")

    task = TrainingTask(
        name=payload.name,
        description=payload.description,
        requirements=payload.requirements,
        course_id=payload.course_id,
        teacher_id=current.id,
        deadline=payload.deadline,
        status="draft",
    )
    db.add(task)
    await db.flush()

    for i, d in enumerate(payload.dimensions):
        db.add(Dimension(task_id=task.id, name=d.name, description=d.description, weight=d.weight, order_index=i))

    await db.commit()
    await db.refresh(task)
    return {"id": task.id, "name": task.name, "status": task.status}


@router.patch("/{task_id}/publish")
async def publish_task(task_id: int, db: DbSession, current: CurrentUser) -> dict[str, object]:
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可操作")
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise ResourceNotFoundError("task not found")
    if task.teacher_id != current.id and current.role != "admin":
        raise AuthorizationError("无权操作他人任务")
    if task.status != "draft":
        raise BusinessRuleError("仅草稿状态可发布", field="status")
    task.status = "published"
    await db.commit()
    return {"id": task.id, "status": "published"}


@router.patch("/{task_id}/close")
async def close_task(task_id: int, db: DbSession, current: CurrentUser) -> dict[str, object]:
    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可操作")
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise ResourceNotFoundError("task not found")
    if task.status != "published":
        raise BusinessRuleError("仅已发布状态可关闭", field="status")
    task.status = "closed"
    await db.commit()
    return {"id": task.id, "status": "closed"}
