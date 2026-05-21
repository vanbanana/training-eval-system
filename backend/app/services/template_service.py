"""TemplateService - 评价模板的可见性、应用与反向保存（Property 19 模板独立性核心）."""

from __future__ import annotations

from sqlalchemy import or_, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.models.task import Dimension
from app.models.template import EvalTemplate, TemplateDimension
from app.models.user import User
from app.repositories.task_repo import DimensionRepository, TaskRepository

log = get_logger(__name__)


class TemplateService:
    def __init__(
        self,
        task_repo: TaskRepository | None = None,
        dim_repo: DimensionRepository | None = None,
    ) -> None:
        self.task_repo = task_repo or TaskRepository()
        self.dim_repo = dim_repo or DimensionRepository()

    @staticmethod
    def _ensure_can_create_system(actor: User, visibility: str) -> None:
        if visibility == "system" and actor.role != "admin":
            raise AuthorizationError("仅管理员可创建系统模板")

    @staticmethod
    def _can_view(actor: User, tpl: EvalTemplate) -> bool:
        if tpl.visibility == "system":
            return True
        if tpl.owner_id == actor.id:
            return True
        if tpl.visibility == "team" and actor.role in {"teacher", "admin"}:
            return True
        return actor.role == "admin"

    async def create_template(
        self,
        db: AsyncSession,
        *,
        actor: User,
        name: str,
        description: str = "",
        visibility: str = "private",
        course_id: int | None = None,
        dimensions: list[dict[str, object]] | None = None,
    ) -> EvalTemplate:
        if actor.role not in {"teacher", "admin"}:
            raise AuthorizationError("仅教师/管理员可创建模板")
        self._ensure_can_create_system(actor, visibility)
        if visibility not in {"private", "team", "system"}:
            raise BusinessRuleError(
                f"非法 visibility {visibility}", field="visibility"
            )

        tpl = EvalTemplate(
            name=name,
            description=description,
            visibility=visibility,
            owner_id=actor.id if visibility != "system" else None,
            course_id=course_id,
        )
        db.add(tpl)
        await db.flush()

        for idx, d in enumerate(dimensions or []):
            db.add(
                TemplateDimension(
                    template_id=tpl.id,
                    name=str(d.get("name", "")),
                    description=str(d.get("description", "")),
                    weight=int(d["weight"]),  # type: ignore[arg-type]
                    order_index=int(d.get("order_index", idx)),  # type: ignore[arg-type]
                )
            )
        await db.flush()
        await db.refresh(tpl)
        log.info(
            "template.created", tpl_id=tpl.id, owner_id=actor.id, visibility=visibility
        )
        return tpl

    async def list_visible(
        self, db: AsyncSession, *, actor: User
    ) -> list[EvalTemplate]:
        """返回该用户可见的所有模板."""
        if actor.role == "admin":
            stmt = select(EvalTemplate).order_by(EvalTemplate.id)
        else:
            stmt = (
                select(EvalTemplate)
                .where(
                    or_(
                        EvalTemplate.visibility == "system",
                        EvalTemplate.owner_id == actor.id,
                        EvalTemplate.visibility == "team",
                    )
                )
                .order_by(EvalTemplate.id)
            )
        return list((await db.execute(stmt)).scalars().all())

    async def get_template(
        self, db: AsyncSession, *, actor: User, template_id: int
    ) -> EvalTemplate:
        tpl = await db.get(EvalTemplate, template_id)
        if tpl is None:
            raise ResourceNotFoundError(f"template {template_id} not found")
        if not self._can_view(actor, tpl):
            raise AuthorizationError("无权访问此模板")
        return tpl

    async def delete_template(
        self, db: AsyncSession, *, actor: User, template_id: int
    ) -> None:
        tpl = await db.get(EvalTemplate, template_id)
        if tpl is None:
            raise ResourceNotFoundError(f"template {template_id} not found")
        if tpl.visibility == "system" and actor.role != "admin":
            raise AuthorizationError("仅管理员可删除系统模板")
        if (
            tpl.visibility != "system"
            and tpl.owner_id != actor.id
            and actor.role != "admin"
        ):
            raise AuthorizationError("无权删除他人模板")
        await db.delete(tpl)
        await db.flush()
        log.info("template.deleted", tpl_id=template_id, actor_id=actor.id)

    async def apply_to_task(
        self, db: AsyncSession, *, actor: User, template_id: int, task_id: int
    ) -> list[Dimension]:
        """将模板维度拷贝到任务（值拷贝，Property 19 保证独立性）."""
        tpl = await self.get_template(db, actor=actor, template_id=template_id)
        task = await self.task_repo.get(db, task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {task_id} not found")
        if task.status != "draft":
            raise BusinessRuleError("仅 draft 任务可应用模板", field="status")
        if actor.role == "teacher" and task.teacher_id != actor.id:
            raise AuthorizationError("无权操作他人任务")

        # 值拷贝：构造 dict 列表传给 replace_all（与 template 实例完全解耦）
        copied = [
            {
                "name": item.name,
                "description": item.description,
                "weight": item.weight,
                "order_index": item.order_index,
            }
            for item in tpl.items
        ]
        new_dims = await self.dim_repo.replace_all_for_task(db, task_id, copied)
        log.info(
            "template.applied",
            tpl_id=template_id,
            task_id=task_id,
            count=len(new_dims),
        )
        return new_dims

    async def save_from_task(
        self,
        db: AsyncSession,
        *,
        actor: User,
        task_id: int,
        name: str,
        description: str = "",
        visibility: str = "private",
        course_id: int | None = None,
    ) -> EvalTemplate:
        """从已有任务的维度反向创建模板."""
        if actor.role not in {"teacher", "admin"}:
            raise AuthorizationError("仅教师/管理员可创建模板")
        self._ensure_can_create_system(actor, visibility)

        task = await self.task_repo.get(db, task_id)
        if task is None:
            raise ResourceNotFoundError(f"task {task_id} not found")
        if actor.role == "teacher" and task.teacher_id != actor.id:
            raise AuthorizationError("无权访问他人任务")

        dims = await self.dim_repo.list_by_task(db, task_id)
        if not dims:
            raise BusinessRuleError("任务无维度，无法保存模板", field="task_id")

        return await self.create_template(
            db,
            actor=actor,
            name=name,
            description=description,
            visibility=visibility,
            course_id=course_id,
            dimensions=[
                {
                    "name": d.name,
                    "description": d.description,
                    "weight": d.weight,
                    "order_index": d.order_index,
                }
                for d in dims
            ],
        )


# ============ 系统预置模板初始化 ============

SYSTEM_TEMPLATES: list[dict[str, object]] = [
    {
        "name": "代码质量评价",
        "description": "聚焦代码规范、文档与功能实现",
        "dimensions": [
            {"name": "代码质量", "weight": 40},
            {"name": "文档完整", "weight": 20},
            {"name": "功能实现", "weight": 40},
        ],
    },
    {
        "name": "文档规范性评价",
        "description": "考察实验报告的结构、内容、表达",
        "dimensions": [
            {"name": "结构清晰", "weight": 30},
            {"name": "内容完整", "weight": 40},
            {"name": "表达规范", "weight": 30},
        ],
    },
    {
        "name": "综合实训评价",
        "description": "代码、文档、功能、创新综合评价",
        "dimensions": [
            {"name": "代码", "weight": 30},
            {"name": "文档", "weight": 20},
            {"name": "功能", "weight": 30},
            {"name": "创新", "weight": 20},
        ],
    },
]


async def seed_system_templates(db: AsyncSession) -> int:
    """启动时调用：插入或确保 3 个系统模板存在；返回新建数量."""
    inserted = 0
    for spec in SYSTEM_TEMPLATES:
        name = str(spec["name"])
        existing = (
            await db.execute(
                select(EvalTemplate).where(
                    EvalTemplate.visibility == "system",
                    EvalTemplate.name == name,
                )
            )
        ).scalar_one_or_none()
        if existing is not None:
            continue

        tpl = EvalTemplate(
            name=name,
            description=str(spec.get("description", "")),
            visibility="system",
            owner_id=None,
        )
        db.add(tpl)
        await db.flush()

        for idx, d in enumerate(spec["dimensions"]):  # type: ignore[index]
            db.add(
                TemplateDimension(
                    template_id=tpl.id,
                    name=str(d["name"]),  # type: ignore[index]
                    weight=int(d["weight"]),  # type: ignore[index, arg-type]
                    order_index=idx,
                )
            )
        inserted += 1

    await db.flush()
    return inserted


template_service = TemplateService()
