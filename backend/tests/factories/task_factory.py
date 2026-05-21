"""TrainingTaskFactory / DimensionFactory."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.models.course import Class
from app.models.task import Dimension, TrainingTask
from app.models.user import User
from tests.factories import faker
from tests.factories.org_factory import ClassFactory, CourseFactory
from tests.factories.user_factory import TeacherFactory


def _split_weights(count: int) -> list[int]:
    """将 100 分割成 count 份正整数（≥5），尽量均分."""
    if count <= 0:
        return []
    base = 100 // count
    remainder = 100 % count
    weights = [base] * count
    for i in range(remainder):
        weights[i] += 1
    return weights


class TrainingTaskFactory:
    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        name: str | None = None,
        teacher: User | None = None,
        teacher_id: int | None = None,
        course_id: int | None = None,
        deadline: datetime | None = None,
        status: str = "published",
        with_dimensions: int | None = None,
        classes: list[Class] | None = None,
        **extra: Any,
    ) -> TrainingTask:
        if teacher is None and teacher_id is None:
            teacher = await TeacherFactory.create_async(
                session, username=f"t_{faker.uuid4()[:6]}"
            )
        if course_id is None:
            course = await CourseFactory.create_async(session)
            course_id = course.id

        task = TrainingTask(
            name=name or f"实训任务-{faker.random_int(min=1, max=999)}",
            description=extra.pop("description", "测试任务"),
            requirements=extra.pop("requirements", "1. 提交报告 2. 提交源码"),
            evaluation_criteria=extra.pop("evaluation_criteria", ""),
            teacher_id=teacher_id or (teacher.id if teacher else 0),
            course_id=course_id,
            status=status,
            deadline=deadline or (datetime.now(UTC) + timedelta(days=7)),
            **extra,
        )
        if classes:
            task.classes = list(classes)
        else:
            # 默认创建一个班级关联
            cls = await ClassFactory.create_async(session, teacher=teacher)
            task.classes = [cls]

        session.add(task)
        await session.flush()
        await session.refresh(task)

        if with_dimensions is not None and with_dimensions > 0:
            weights = _split_weights(with_dimensions)
            for i, w in enumerate(weights):
                session.add(
                    Dimension(
                        task_id=task.id,
                        name=f"维度 {i + 1}",
                        weight=w,
                        order_index=i,
                    )
                )
            await session.flush()
            await session.refresh(task)

        return task


class DimensionFactory:
    _counter = 0

    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        task: TrainingTask | None = None,
        task_id: int | None = None,
        name: str | None = None,
        weight: int = 50,
        order_index: int | None = None,
        **extra: Any,
    ) -> Dimension:
        cls._counter += 1
        dim = Dimension(
            task_id=task_id or (task.id if task else 0),
            name=name or f"维度 {cls._counter}",
            weight=weight,
            order_index=order_index if order_index is not None else cls._counter,
            **extra,
        )
        session.add(dim)
        await session.flush()
        await session.refresh(dim)
        return dim
