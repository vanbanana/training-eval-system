"""TaskService - 任务状态机、维度配置、字段锁定权限边界."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
from typing import TypedDict

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import (
    AuthorizationError,
    DeadlineInvalidError,
    DimensionCountInvalidError,
    DimensionsLockedError,
    DimensionWeightTooLowError,
    FieldLockedError,
    InvalidStatusTransitionError,
    ResourceNotFoundError,
    WeightSumInvalidError,
)
from app.core.logging import get_logger
from app.models.task import TrainingTask
from app.models.user import User
from app.repositories.org_repo import ClassRepository
from app.repositories.task_repo import DimensionRepository, TaskRepository

log = get_logger(__name__)


MIN_DIMENSIONS = 2
MAX_DIMENSIONS = 10
MIN_WEIGHT_PER_DIMENSION = 5
DRAFT, PUBLISHED, CLOSED = "draft", "published", "closed"

# 各状态允许编辑的字段
_EDITABLE_FIELDS_BY_STATUS: dict[str, set[str]] = {
    DRAFT: {
        "name",
        "description",
        "requirements",
        "evaluation_criteria",
        "deadline",
        "course_id",
    },
    PUBLISHED: {"description", "requirements", "deadline", "evaluation_criteria"},
    CLOSED: set(),
}


class DimensionInput(TypedDict, total=False):
    name: str
    description: str
    weight: int
    order_index: int


class CreateTaskData(TypedDict, total=False):
    name: str
    description: str
    requirements: str
    evaluation_criteria: str
    course_id: int
    deadline: datetime | None
    class_ids: list[int]


@dataclass(slots=True)
class _DimensionValidation:
    total_weight: int
    count: int


class TaskService:
    def __init__(
        self,
        task_repo: TaskRepository | None = None,
        dim_repo: DimensionRepository | None = None,
        class_repo: ClassRepository | None = None,
    ) -> None:
        self.task_repo = task_repo or TaskRepository()
        self.dim_repo = dim_repo or DimensionRepository()
        self.class_repo = class_repo or ClassRepository()

    # ============ 创建 / 编辑 / 状态转移 ============

    async def create_task(
        self, db: AsyncSession, *, actor: User, data: CreateTaskData
    ) -> TrainingTask:
        if actor.role not in {"teacher", "admin"}:
            raise AuthorizationError("仅教师可创建任务")
        task = TrainingTask(
            name=data["name"],
            description=data.get("description", ""),
            requirements=data.get("requirements", ""),
            evaluation_criteria=data.get("evaluation_criteria", ""),
            teacher_id=actor.id,
            course_id=data["course_id"],
            status=DRAFT,
            deadline=data.get("deadline"),
        )
        # 关联班级
        class_ids = data.get("class_ids") or []
        if class_ids:
            for cid in class_ids:
                cls = await self.class_repo.get(db, cid)
                if cls is not None:
                    task.classes.append(cls)
        db.add(task)
        await db.flush()
        await db.refresh(task)
        log.info(
            "task.created",
            task_id=task.id,
            teacher_id=actor.id,
            class_count=len(task.classes),
        )
        return task

    async def update_task(
        self,
        db: AsyncSession,
        *,
        actor: User,
        task_id: int,
        data: dict[str, object],
    ) -> TrainingTask:
        task = await self._get_task_for_actor(db, actor=actor, task_id=task_id)

        editable = _EDITABLE_FIELDS_BY_STATUS[task.status]
        for field in data:
            if field not in editable:
                raise FieldLockedError(
                    f"字段 {field} 在 {task.status} 状态下被锁定",
                    field=field,
                )

        for field, value in data.items():
            setattr(task, field, value)

        await db.flush()
        await db.refresh(task)
        return task

    async def publish_task(
        self, db: AsyncSession, *, actor: User, task_id: int
    ) -> TrainingTask:
        task = await self._get_task_for_actor(db, actor=actor, task_id=task_id)

        if task.status != DRAFT:
            raise InvalidStatusTransitionError(
                f"仅 draft 状态可发布，当前状态 {task.status}", field="status"
            )

        # 校验：维度数 / 权重和
        dims = await self.dim_repo.list_by_task(db, task_id)
        if not (MIN_DIMENSIONS <= len(dims) <= MAX_DIMENSIONS):
            raise DimensionCountInvalidError(
                f"维度数必须在 {MIN_DIMENSIONS}-{MAX_DIMENSIONS} 之间，当前 {len(dims)}",
                field="dimensions",
            )
        total = sum(d.weight for d in dims)
        if total != 100:
            raise WeightSumInvalidError(
                f"维度权重总和必须为 100，当前 {total}", field="dimensions"
            )

        # 校验：deadline 必须 > now
        if task.deadline is not None:
            now = datetime.now(UTC)
            deadline = task.deadline
            if deadline.tzinfo is None:
                deadline = deadline.replace(tzinfo=UTC)
            if deadline <= now:
                raise DeadlineInvalidError(
                    "截止时间必须晚于当前时间", field="deadline"
                )

        # 校验：至少关联 1 个 class
        if not task.classes:
            raise DimensionCountInvalidError(
                "任务必须关联至少一个班级", field="class_ids"
            )

        task.status = PUBLISHED
        await db.flush()
        await db.refresh(task)
        log.info("task.published", task_id=task.id, actor_id=actor.id)
        return task

    async def close_task(
        self, db: AsyncSession, *, actor: User, task_id: int
    ) -> TrainingTask:
        task = await self._get_task_for_actor(db, actor=actor, task_id=task_id)
        if task.status == CLOSED:
            return task  # 幂等
        if task.status != PUBLISHED:
            raise InvalidStatusTransitionError(
                f"仅 published 状态可关闭，当前状态 {task.status}", field="status"
            )
        task.status = CLOSED
        await db.flush()
        await db.refresh(task)
        log.info("task.closed", task_id=task.id, actor_id=actor.id)
        return task

    async def auto_close_expired_tasks(
        self, db: AsyncSession, *, now: datetime | None = None
    ) -> int:
        """系统调用：将所有 deadline 已过期的 published 任务关闭；返回关闭数量."""
        from sqlalchemy import select

        check_time = now or datetime.now(UTC)
        stmt = select(TrainingTask).where(
            TrainingTask.status == PUBLISHED,
            TrainingTask.deadline.is_not(None),
            TrainingTask.deadline < check_time,
        )
        tasks = list((await db.execute(stmt)).scalars().all())
        for t in tasks:
            t.status = CLOSED
        await db.flush()
        log.info("task.auto_close.batch", count=len(tasks))
        return len(tasks)

    # ============ 维度配置 ============

    async def set_dimensions(
        self,
        db: AsyncSession,
        *,
        actor: User,
        task_id: int,
        dimensions: list[DimensionInput],
    ) -> list[object]:
        task = await self._get_task_for_actor(db, actor=actor, task_id=task_id)
        if task.status != DRAFT:
            raise DimensionsLockedError(
                f"任务状态 {task.status} 不允许修改维度", field="status"
            )

        # 校验
        if not (MIN_DIMENSIONS <= len(dimensions) <= MAX_DIMENSIONS):
            raise DimensionCountInvalidError(
                f"维度数必须在 {MIN_DIMENSIONS}-{MAX_DIMENSIONS} 之间",
                field="dimensions",
            )
        total = sum(int(d["weight"]) for d in dimensions)  # type: ignore[index]
        if total != 100:
            raise WeightSumInvalidError(
                f"维度权重总和必须为 100，当前 {total}", field="dimensions"
            )
        for d in dimensions:
            w = int(d.get("weight", 0))  # type: ignore[arg-type]
            if w < MIN_WEIGHT_PER_DIMENSION:
                raise DimensionWeightTooLowError(
                    f"单维度权重必须 ≥ {MIN_WEIGHT_PER_DIMENSION}", field="weight"
                )

        # 替换
        new_dims = await self.dim_repo.replace_all_for_task(
            db, task_id, [dict(d) for d in dimensions]
        )
        log.info(
            "task.dimensions.updated",
            task_id=task_id,
            count=len(new_dims),
            actor_id=actor.id,
        )
        return new_dims

    # ============ 私有辅助 ============

    async def _get_task_for_actor(
        self, db: AsyncSession, *, actor: User, task_id: int
    ) -> TrainingTask:
        task = await self.task_repo.get(db, task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {task_id} not found")
        if actor.role == "teacher" and task.teacher_id != actor.id:
            raise AuthorizationError("无权操作他人任务")
        return task


task_service = TaskService()
