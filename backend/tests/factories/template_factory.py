"""EvaluationTemplateFactory."""

from __future__ import annotations

from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.models.template import EvalTemplate, TemplateDimension
from app.models.user import User
from tests.factories import faker
from tests.factories.user_factory import TeacherFactory


def _split_weights(count: int) -> list[int]:
    base = 100 // count
    rem = 100 % count
    return [base + (1 if i < rem else 0) for i in range(count)]


class EvaluationTemplateFactory:
    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        name: str | None = None,
        owner: User | None = None,
        owner_id: int | None = None,
        visibility: str = "private",
        course_id: int | None = None,
        with_dimensions: int = 3,
        **extra: Any,
    ) -> EvalTemplate:
        if visibility != "system":
            if owner is None and owner_id is None:
                owner = await TeacherFactory.create_async(
                    session, username=f"to_{faker.uuid4()[:6]}"
                )
            owner_id = owner_id or (owner.id if owner else None)
        else:
            owner_id = None

        tpl = EvalTemplate(
            name=name or f"模板-{faker.random_int(1, 999)}",
            description=extra.pop("description", ""),
            visibility=visibility,
            owner_id=owner_id,
            course_id=course_id,
            **extra,
        )
        session.add(tpl)
        await session.flush()
        await session.refresh(tpl)

        if with_dimensions > 0:
            for i, w in enumerate(_split_weights(with_dimensions)):
                session.add(
                    TemplateDimension(
                        template_id=tpl.id,
                        name=f"维度 {i + 1}",
                        weight=w,
                        order_index=i,
                    )
                )
            await session.flush()
            await session.refresh(tpl)
        return tpl


class SystemTemplateFactory(EvaluationTemplateFactory):
    @classmethod
    async def create_async(  # type: ignore[override]
        cls,
        session: AsyncSession,
        *,
        name: str | None = None,
        with_dimensions: int = 3,
        **extra: Any,
    ) -> EvalTemplate:
        return await super().create_async(
            session,
            name=name,
            visibility="system",
            with_dimensions=with_dimensions,
            **extra,
        )
