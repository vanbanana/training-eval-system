"""Task 5.3 / 5.4 / 5.8 验收：TaskService 状态机 + 维度 + 字段锁."""

from __future__ import annotations

from collections.abc import AsyncIterator
from datetime import UTC, datetime, timedelta

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import (
    AuthorizationError,
    DeadlineInvalidError,
    DimensionCountInvalidError,
    DimensionWeightTooLowError,
    DimensionsLockedError,
    FieldLockedError,
    InvalidStatusTransitionError,
    WeightSumInvalidError,
)
from app.services.task_service import TaskService
from tests.factories.org_factory import ClassFactory, CourseFactory
from tests.factories.user_factory import (
    AdminFactory,
    TeacherFactory,
    UserFactory,
)


pytestmark = pytest.mark.unit


@pytest.fixture()
async def session() -> AsyncIterator[AsyncSession]:
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.execute(text("PRAGMA foreign_keys = ON"))
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


@pytest.fixture()
def svc() -> TaskService:
    return TaskService()


@pytest.fixture()
async def setup_basic(session: AsyncSession) -> dict[str, object]:
    """提供 teacher / course / class / draft task."""
    teacher = await TeacherFactory.create_async(session, username="t-task")
    course = await CourseFactory.create_async(session)
    cls = await ClassFactory.create_async(session, teacher=teacher, course=course)
    await session.commit()
    return {"teacher": teacher, "course": course, "class": cls}


class TestCreateTask:
    async def test_creates_draft(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]

        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "实训 1",
                "description": "测试",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await session.commit()
        assert task.status == "draft"
        assert task.teacher_id == teacher.id
        assert len(task.classes) == 1

    async def test_student_cannot_create(
        self, session: AsyncSession, svc: TaskService
    ) -> None:
        student = await UserFactory.create_async(session, username="ssc")
        course = await CourseFactory.create_async(session)
        await session.commit()
        with pytest.raises(AuthorizationError):
            await svc.create_task(
                session,
                actor=student,
                data={"name": "X", "course_id": course.id},
            )


class TestPublishTask:
    async def test_publish_with_valid_state(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "代码", "weight": 50},
                {"name": "报告", "weight": 50},
            ],
        )
        await session.commit()

        published = await svc.publish_task(session, actor=teacher, task_id=task.id)
        assert published.status == "published"

    async def test_weight_sum_99_raises(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        # 直接绕过 service 校验插入维度
        from app.models.task import Dimension

        session.add(Dimension(task_id=task.id, name="A", weight=49))
        session.add(Dimension(task_id=task.id, name="B", weight=50))
        await session.commit()

        with pytest.raises(WeightSumInvalidError):
            await svc.publish_task(session, actor=teacher, task_id=task.id)

    async def test_dimension_count_too_few(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        from app.models.task import Dimension

        session.add(Dimension(task_id=task.id, name="A", weight=100))
        await session.commit()

        with pytest.raises(DimensionCountInvalidError):
            await svc.publish_task(session, actor=teacher, task_id=task.id)

    async def test_deadline_in_past_rejected(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) - timedelta(hours=1),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await session.commit()

        with pytest.raises(DeadlineInvalidError):
            await svc.publish_task(session, actor=teacher, task_id=task.id)

    async def test_no_classes_rejected(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await session.commit()

        with pytest.raises(DimensionCountInvalidError):
            await svc.publish_task(session, actor=teacher, task_id=task.id)


class TestCloseTask:
    async def test_close_published(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await session.commit()

        closed = await svc.close_task(session, actor=teacher, task_id=task.id)
        assert closed.status == "closed"

    async def test_close_draft_raises(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={"name": "X", "course_id": course.id, "class_ids": [cls.id]},
        )
        await session.commit()
        with pytest.raises(InvalidStatusTransitionError):
            await svc.close_task(session, actor=teacher, task_id=task.id)

    async def test_close_already_closed_idempotent(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await svc.close_task(session, actor=teacher, task_id=task.id)
        await session.commit()
        # 二次关闭不抛
        result = await svc.close_task(session, actor=teacher, task_id=task.id)
        assert result.status == "closed"


class TestSetDimensions:
    async def test_low_weight_rejected(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={"name": "X", "course_id": course.id, "class_ids": [cls.id]},
        )
        await session.commit()

        with pytest.raises(DimensionWeightTooLowError):
            await svc.set_dimensions(
                session,
                actor=teacher,
                task_id=task.id,
                dimensions=[
                    {"name": "A", "weight": 4},
                    {"name": "B", "weight": 96},
                ],
            )

    async def test_locked_when_published(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await session.commit()

        with pytest.raises(DimensionsLockedError):
            await svc.set_dimensions(
                session,
                actor=teacher,
                task_id=task.id,
                dimensions=[
                    {"name": "X", "weight": 100},
                ],
            )


class TestFieldLocking:
    """Task 5.8 - 编辑权限边界."""

    async def test_draft_allows_name_change(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={"name": "Old", "course_id": course.id, "class_ids": [cls.id]},
        )
        updated = await svc.update_task(
            session, actor=teacher, task_id=task.id, data={"name": "New"}
        )
        assert updated.name == "New"

    async def test_published_locks_name(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "Old",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await session.commit()

        with pytest.raises(FieldLockedError) as exc:
            await svc.update_task(
                session, actor=teacher, task_id=task.id, data={"name": "X"}
            )
        assert exc.value.field == "name"

    async def test_published_allows_description(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await session.commit()

        updated = await svc.update_task(
            session,
            actor=teacher,
            task_id=task.id,
            data={"description": "新描述"},
        )
        assert updated.description == "新描述"

    async def test_closed_locks_all(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await svc.close_task(session, actor=teacher, task_id=task.id)
        await session.commit()

        for field in ("name", "description", "deadline"):
            with pytest.raises(FieldLockedError):
                await svc.update_task(
                    session, actor=teacher, task_id=task.id, data={field: "x"}
                )


class TestAutoClose:
    async def test_auto_close_expired(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        teacher = setup_basic["teacher"]
        course = setup_basic["course"]
        cls = setup_basic["class"]
        task = await svc.create_task(
            session,
            actor=teacher,
            data={
                "name": "X",
                "course_id": course.id,
                "class_ids": [cls.id],
                "deadline": datetime.now(UTC) + timedelta(days=7),
            },
        )
        await svc.set_dimensions(
            session,
            actor=teacher,
            task_id=task.id,
            dimensions=[
                {"name": "A", "weight": 50},
                {"name": "B", "weight": 50},
            ],
        )
        await svc.publish_task(session, actor=teacher, task_id=task.id)
        await session.commit()

        # 推进 now 到 deadline 之后
        future = datetime.now(UTC) + timedelta(days=8)
        n = await svc.auto_close_expired_tasks(session, now=future)
        await session.commit()
        assert n == 1

        await session.refresh(task)
        assert task.status == "closed"

    async def test_auto_close_idempotent(
        self,
        session: AsyncSession,
        svc: TaskService,
        setup_basic: dict[str, object],
    ) -> None:
        # 没有过期任务时返回 0
        n = await svc.auto_close_expired_tasks(session)
        await session.commit()
        assert n == 0


class TestAuthorization:
    async def test_other_teacher_cannot_edit(
        self, session: AsyncSession, svc: TaskService
    ) -> None:
        ta = await TeacherFactory.create_async(session, username="own-T")
        tb = await TeacherFactory.create_async(session, username="other-T")
        course = await CourseFactory.create_async(session)
        cls = await ClassFactory.create_async(session, teacher=ta, course=course)
        await session.commit()

        task = await svc.create_task(
            session,
            actor=ta,
            data={"name": "X", "course_id": course.id, "class_ids": [cls.id]},
        )
        await session.commit()

        with pytest.raises(AuthorizationError):
            await svc.update_task(
                session, actor=tb, task_id=task.id, data={"name": "tampered"}
            )

    async def test_admin_can_edit_any(
        self, session: AsyncSession, svc: TaskService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="own-A")
        admin = await AdminFactory.create_async(session, username="adm")
        course = await CourseFactory.create_async(session)
        cls = await ClassFactory.create_async(session, teacher=teacher, course=course)
        await session.commit()

        task = await svc.create_task(
            session,
            actor=teacher,
            data={"name": "X", "course_id": course.id, "class_ids": [cls.id]},
        )
        await session.commit()

        # admin 应可编辑
        updated = await svc.update_task(
            session, actor=admin, task_id=task.id, data={"name": "by admin"}
        )
        assert updated.name == "by admin"
