"""Task 6.2 / 6.3 验收：TemplateService（含 Property 19 模板独立性）."""

from __future__ import annotations

from collections.abc import AsyncIterator

import pytest
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.core.database import Base
from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    ResourceNotFoundError,
)
from app.repositories.task_repo import DimensionRepository
from app.services.template_service import (
    TemplateService,
    seed_system_templates,
)
from tests.factories.org_factory import CourseFactory
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.template_factory import EvaluationTemplateFactory
from tests.factories.user_factory import (
    AdminFactory,
    TeacherFactory,
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
def svc() -> TemplateService:
    return TemplateService()


class TestVisibility:
    async def test_admin_sees_all(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        admin = await AdminFactory.create_async(session, username="adm")
        ta = await TeacherFactory.create_async(session, username="ta")
        await session.commit()

        await EvaluationTemplateFactory.create_async(
            session, owner=ta, visibility="private"
        )
        await EvaluationTemplateFactory.create_async(session, visibility="system")
        await session.commit()

        result = await svc.list_visible(session, actor=admin)
        assert len(result) == 2

    async def test_teacher_cannot_see_other_private(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        ta = await TeacherFactory.create_async(session, username="ta1")
        tb = await TeacherFactory.create_async(session, username="tb1")
        await session.commit()

        await EvaluationTemplateFactory.create_async(
            session, owner=ta, visibility="private", name="A 私有"
        )
        await EvaluationTemplateFactory.create_async(
            session, owner=tb, visibility="private", name="B 私有"
        )
        await EvaluationTemplateFactory.create_async(
            session, visibility="system", name="系统"
        )
        await session.commit()

        a_visible = await svc.list_visible(session, actor=ta)
        names = {t.name for t in a_visible}
        assert "A 私有" in names
        assert "系统" in names
        assert "B 私有" not in names


class TestCreate:
    async def test_create_private_with_dimensions(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="tc")
        await session.commit()

        tpl = await svc.create_template(
            session,
            actor=teacher,
            name="X",
            visibility="private",
            dimensions=[
                {"name": "A", "weight": 60},
                {"name": "B", "weight": 40},
            ],
        )
        await session.commit()
        assert tpl.id is not None
        assert tpl.owner_id == teacher.id
        assert len(tpl.items) == 2

    async def test_teacher_cannot_create_system(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="ts")
        await session.commit()

        with pytest.raises(AuthorizationError):
            await svc.create_template(
                session, actor=teacher, name="X", visibility="system"
            )


class TestApply:
    async def test_apply_copies_dimensions_independently(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        """Property 19：apply 后修改 task dimension 不影响 template."""
        teacher = await TeacherFactory.create_async(session, username="tap")
        await session.commit()

        # 模板：3 维度
        tpl = await EvaluationTemplateFactory.create_async(
            session, owner=teacher, with_dimensions=3
        )
        await session.commit()

        # 任务（draft）
        task = await TrainingTaskFactory.create_async(
            session, teacher=teacher, status="draft", with_dimensions=None
        )
        await session.commit()

        # 应用模板
        new_dims = await svc.apply_to_task(
            session, actor=teacher, template_id=tpl.id, task_id=task.id
        )
        await session.commit()
        assert len(new_dims) == 3

        # 修改 task 的某维度
        repo = DimensionRepository()
        task_dims = await repo.list_by_task(session, task.id)
        task_dims[0].weight = 99
        task_dims[0].name = "已篡改"
        await session.commit()

        # 验证 template 未变
        await session.refresh(tpl)
        original_weights = sorted([item.weight for item in tpl.items])
        assert sum(original_weights) == 100
        assert all(item.name != "已篡改" for item in tpl.items)

    async def test_apply_to_published_rejected(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="tap2")
        await session.commit()
        tpl = await EvaluationTemplateFactory.create_async(
            session, owner=teacher
        )
        # published 任务
        task = await TrainingTaskFactory.create_async(
            session,
            teacher=teacher,
            status="published",
            with_dimensions=2,
        )
        await session.commit()

        with pytest.raises(BusinessRuleError) as exc:
            await svc.apply_to_task(
                session, actor=teacher, template_id=tpl.id, task_id=task.id
            )
        assert exc.value.field == "status"

    async def test_apply_to_other_teacher_task_rejected(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        ta = await TeacherFactory.create_async(session, username="tap-a")
        tb = await TeacherFactory.create_async(session, username="tap-b")
        await session.commit()

        tpl = await EvaluationTemplateFactory.create_async(
            session, owner=ta, visibility="team"
        )
        task = await TrainingTaskFactory.create_async(
            session,
            teacher=tb,
            status="draft",
            with_dimensions=None,
        )
        await session.commit()

        # ta 操作 tb 的任务
        with pytest.raises(AuthorizationError):
            await svc.apply_to_task(
                session, actor=ta, template_id=tpl.id, task_id=task.id
            )


class TestDeleteTemplate:
    async def test_delete_does_not_affect_applied_task(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        """Property 19：删除模板后已应用的任务仍正常工作."""
        teacher = await TeacherFactory.create_async(session, username="td")
        await session.commit()
        tpl = await EvaluationTemplateFactory.create_async(
            session, owner=teacher, with_dimensions=3
        )
        task = await TrainingTaskFactory.create_async(
            session,
            teacher=teacher,
            status="draft",
            with_dimensions=None,
        )
        await session.commit()

        await svc.apply_to_task(
            session, actor=teacher, template_id=tpl.id, task_id=task.id
        )
        await session.commit()
        tpl_id = tpl.id

        # 删除模板
        await svc.delete_template(
            session, actor=teacher, template_id=tpl_id
        )
        await session.commit()

        # 任务仍可正常读取，维度数量 = 3
        repo = DimensionRepository()
        dims = await repo.list_by_task(session, task.id)
        assert len(dims) == 3

    async def test_other_user_cannot_delete(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        ta = await TeacherFactory.create_async(session, username="tdel-a")
        tb = await TeacherFactory.create_async(session, username="tdel-b")
        await session.commit()
        tpl = await EvaluationTemplateFactory.create_async(session, owner=ta)
        await session.commit()

        with pytest.raises(AuthorizationError):
            await svc.delete_template(
                session, actor=tb, template_id=tpl.id
            )


class TestSaveFromTask:
    async def test_save_creates_template_with_same_dimensions(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        teacher = await TeacherFactory.create_async(session, username="sf")
        await session.commit()
        task = await TrainingTaskFactory.create_async(
            session, teacher=teacher, with_dimensions=3, status="draft"
        )
        await session.commit()

        tpl = await svc.save_from_task(
            session,
            actor=teacher,
            task_id=task.id,
            name="From task",
            visibility="private",
        )
        await session.commit()

        assert tpl.id is not None
        assert len(tpl.items) == 3
        assert sum(it.weight for it in tpl.items) == 100


class TestSeedSystemTemplates:
    async def test_seed_inserts_three_templates(
        self, session: AsyncSession
    ) -> None:
        n = await seed_system_templates(session)
        await session.commit()
        assert n == 3

        # 重复调用不会再插入
        n2 = await seed_system_templates(session)
        await session.commit()
        assert n2 == 0

    async def test_any_teacher_sees_seeded_templates(
        self, session: AsyncSession, svc: TemplateService
    ) -> None:
        await seed_system_templates(session)
        await session.commit()

        teacher = await TeacherFactory.create_async(session, username="see")
        await session.commit()

        result = await svc.list_visible(session, actor=teacher)
        names = {t.name for t in result if t.visibility == "system"}
        assert "代码质量评价" in names
        assert "文档规范性评价" in names
        assert "综合实训评价" in names
