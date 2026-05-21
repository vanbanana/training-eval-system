"""实训任务路由."""

from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter
from pydantic import BaseModel, Field
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import ResourceNotFoundError
from app.models.course import ClassMembership
from app.models.task import TrainingTask
from app.repositories.task_repo import TaskRepository
from app.schemas.task import TaskOut
from app.services.task_service import task_service

router = APIRouter(prefix="/api/tasks", tags=["tasks"])

_task_repo = TaskRepository()


class CreateTaskRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    description: str = ""
    requirements: str = ""
    evaluation_criteria: str = ""
    course_id: int
    class_ids: list[int] = Field(default_factory=list)
    deadline: datetime | None = None


class UpdateTaskRequest(BaseModel):
    name: str | None = Field(default=None, min_length=1, max_length=100)
    description: str | None = None
    requirements: str | None = None
    evaluation_criteria: str | None = None
    deadline: datetime | None = None


class DimensionInput(BaseModel):
    name: str = Field(..., min_length=1, max_length=64)
    description: str = ""
    weight: int = Field(..., ge=1, le=100)


class SetDimensionsRequest(BaseModel):
    dimensions: list[DimensionInput] = Field(..., min_length=1, max_length=20)


@router.get("", response_model=list[TaskOut])
async def list_tasks(db: DbSession, current: CurrentUser) -> list[TaskOut]:
    """教师看自己的；学生看所属班级关联的 published 任务；管理员看全部."""
    if current.role == "admin":
        tasks = await _task_repo.list(db, limit=200)
    elif current.role == "teacher":
        tasks = await _task_repo.list_by_teacher(db, current.id)
    else:
        # 学生：先取班级 ID，再查关联的 published 任务
        memberships = (
            await db.execute(
                select(ClassMembership.class_id).where(
                    ClassMembership.student_id == current.id
                )
            )
        ).scalars().all()
        tasks = await _task_repo.list_by_classes_for_student(
            db, list(memberships), status="published"
        )
    return [TaskOut.model_validate(t, from_attributes=True) for t in tasks]


@router.get("/{task_id}", response_model=TaskOut)
async def get_task(
    task_id: int, db: DbSession, current: CurrentUser
) -> TaskOut:
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise ResourceNotFoundError(f"task {task_id} not found")
    return TaskOut.model_validate(task, from_attributes=True)


@router.post("", status_code=201)
async def create_task(
    payload: CreateTaskRequest, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    task = await task_service.create_task(
        db,
        actor=current,
        data={
            "name": payload.name,
            "description": payload.description,
            "requirements": payload.requirements,
            "evaluation_criteria": payload.evaluation_criteria,
            "course_id": payload.course_id,
            "class_ids": payload.class_ids,
            "deadline": payload.deadline,
        },
    )
    return {"id": task.id, "name": task.name, "status": task.status}


@router.patch("/{task_id}")
async def update_task(
    task_id: int,
    payload: UpdateTaskRequest,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    data = payload.model_dump(exclude_unset=True)
    task = await task_service.update_task(
        db, actor=current, task_id=task_id, data=data
    )
    return {"id": task.id, "status": task.status, "updated": True}


@router.delete("/{task_id}", status_code=204)
async def delete_task(
    task_id: int, db: DbSession, current: CurrentUser
) -> None:
    """删除任务（仅 draft 状态可删）."""
    from app.core.exceptions import (
        AuthorizationError,
        BusinessRuleError,
    )
    from app.core.exceptions import (
        ResourceNotFoundError as RNF,
    )

    if current.role not in ("teacher", "admin"):
        raise AuthorizationError("仅教师可删除任务")
    task = await db.get(TrainingTask, task_id)
    if not task:
        raise RNF(f"task {task_id} not found")
    if current.role == "teacher" and task.teacher_id != current.id:
        raise AuthorizationError("无权操作他人任务")
    if task.status != "draft":
        raise BusinessRuleError("仅草稿状态可删除", field="status")
    await db.delete(task)
    await db.flush()


@router.post("/{task_id}/publish")
async def publish_task(
    task_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    task = await task_service.publish_task(
        db, actor=current, task_id=task_id
    )
    return {"id": task.id, "status": task.status}


@router.post("/{task_id}/close")
async def close_task(
    task_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    task = await task_service.close_task(db, actor=current, task_id=task_id)
    return {"id": task.id, "status": task.status}


@router.put("/{task_id}/dimensions")
async def set_dimensions(
    task_id: int,
    payload: SetDimensionsRequest,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    dims = await task_service.set_dimensions(
        db,
        actor=current,
        task_id=task_id,
        dimensions=[d.model_dump() for d in payload.dimensions],
    )
    return {"task_id": task_id, "count": len(dims)}
