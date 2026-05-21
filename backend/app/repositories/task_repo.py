"""TaskRepository / DimensionRepository - 实训任务数据访问."""

from __future__ import annotations

from sqlalchemy import delete, func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.task import Dimension, TrainingTask, task_classes
from app.repositories.base import BaseRepository


class TaskRepository(BaseRepository[TrainingTask]):
    model = TrainingTask

    async def list_by_teacher(
        self,
        db: AsyncSession,
        teacher_id: int,
        *,
        status: str | None = None,
    ) -> list[TrainingTask]:
        stmt = select(TrainingTask).where(TrainingTask.teacher_id == teacher_id)
        if status is not None:
            stmt = stmt.where(TrainingTask.status == status)
        stmt = stmt.order_by(TrainingTask.created_at.desc())
        return list((await db.execute(stmt)).scalars().all())

    async def list_by_class(
        self,
        db: AsyncSession,
        class_id: int,
        *,
        status: str | None = None,
    ) -> list[TrainingTask]:
        stmt = (
            select(TrainingTask)
            .join(task_classes, task_classes.c.task_id == TrainingTask.id)
            .where(task_classes.c.class_id == class_id)
        )
        if status is not None:
            stmt = stmt.where(TrainingTask.status == status)
        stmt = stmt.order_by(TrainingTask.created_at.desc())
        return list((await db.execute(stmt)).scalars().all())

    async def list_by_classes_for_student(
        self,
        db: AsyncSession,
        class_ids: list[int],
        *,
        status: str | None = "published",
    ) -> list[TrainingTask]:
        if not class_ids:
            return []
        stmt = (
            select(TrainingTask)
            .join(task_classes, task_classes.c.task_id == TrainingTask.id)
            .where(task_classes.c.class_id.in_(class_ids))
        )
        if status is not None:
            stmt = stmt.where(TrainingTask.status == status)
        stmt = stmt.order_by(TrainingTask.created_at.desc())
        result = await db.execute(stmt)
        # 去重（学生多班可能重复关联）
        seen = set()
        ordered: list[TrainingTask] = []
        for t in result.scalars().all():
            if t.id not in seen:
                seen.add(t.id)
                ordered.append(t)
        return ordered


class DimensionRepository(BaseRepository[Dimension]):
    model = Dimension

    async def list_by_task(
        self, db: AsyncSession, task_id: int
    ) -> list[Dimension]:
        stmt = (
            select(Dimension)
            .where(Dimension.task_id == task_id)
            .order_by(Dimension.order_index, Dimension.id)
        )
        return list((await db.execute(stmt)).scalars().all())

    async def sum_weights(self, db: AsyncSession, task_id: int) -> int:
        stmt = select(func.coalesce(func.sum(Dimension.weight), 0)).where(
            Dimension.task_id == task_id
        )
        result = await db.execute(stmt)
        return int(result.scalar_one())

    async def replace_all_for_task(
        self,
        db: AsyncSession,
        task_id: int,
        dimensions: list[dict[str, object]],
    ) -> list[Dimension]:
        """原子替换：先删除该 task 的全部维度，再批量插入新的.

        调用方负责事务边界（commit 或 rollback）。
        """
        await db.execute(delete(Dimension).where(Dimension.task_id == task_id))
        new_dims: list[Dimension] = []
        for idx, item in enumerate(dimensions):
            dim = Dimension(
                task_id=task_id,
                name=str(item.get("name", "")),
                description=str(item.get("description", "")),
                weight=int(item["weight"]),  # type: ignore[arg-type]
                order_index=int(item.get("order_index", idx)),  # type: ignore[arg-type]
            )
            db.add(dim)
            new_dims.append(dim)
        await db.flush()
        return new_dims
