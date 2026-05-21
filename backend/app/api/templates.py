"""模板路由."""

from __future__ import annotations

from fastapi import APIRouter

from app.api.deps import CurrentUser, DbSession
from app.schemas.template import (
    ApplyTemplateRequest,
    CreateTemplateRequest,
    SaveFromTaskRequest,
    TemplateOut,
)
from app.services.template_service import template_service

router = APIRouter(prefix="/api/templates", tags=["templates"])


@router.get("", response_model=list[TemplateOut])
async def list_templates(
    db: DbSession, current: CurrentUser
) -> list[TemplateOut]:
    items = await template_service.list_visible(db, actor=current)
    return [TemplateOut.model_validate(t, from_attributes=True) for t in items]


@router.post("", response_model=TemplateOut, status_code=201)
async def create_template(
    payload: CreateTemplateRequest, db: DbSession, current: CurrentUser
) -> TemplateOut:
    tpl = await template_service.create_template(
        db,
        actor=current,
        name=payload.name,
        description=payload.description,
        visibility=payload.visibility,
        course_id=payload.course_id,
        dimensions=[d.model_dump() for d in payload.dimensions],
    )
    return TemplateOut.model_validate(tpl, from_attributes=True)


@router.delete("/{template_id}", status_code=204)
async def delete_template(
    template_id: int, db: DbSession, current: CurrentUser
) -> None:
    await template_service.delete_template(
        db, actor=current, template_id=template_id
    )


@router.post("/{template_id}/apply")
async def apply_template(
    template_id: int,
    payload: ApplyTemplateRequest,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    new_dims = await template_service.apply_to_task(
        db, actor=current, template_id=template_id, task_id=payload.task_id
    )
    return {"task_id": payload.task_id, "applied_count": len(new_dims)}


@router.post("/from-task", response_model=TemplateOut, status_code=201)
async def save_from_task(
    payload: SaveFromTaskRequest, db: DbSession, current: CurrentUser
) -> TemplateOut:
    tpl = await template_service.save_from_task(
        db,
        actor=current,
        task_id=payload.task_id,
        name=payload.name,
        description=payload.description,
        visibility=payload.visibility,
        course_id=payload.course_id,
    )
    return TemplateOut.model_validate(tpl, from_attributes=True)
