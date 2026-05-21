"""班级管理路由."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import AuthorizationError, ResourceNotFoundError
from app.models.course import Class
from app.models.user import User
from app.repositories.org_repo import MembershipRepository
from app.services.org_service import org_service

router = APIRouter(prefix="/api/classes", tags=["classes"])

_membership_repo = MembershipRepository()


@router.get("")
async def list_my_classes(
    db: DbSession, current: CurrentUser
) -> list[dict[str, object]]:
    """教师看自己的班级，管理员看全部."""
    stmt = select(Class)
    if current.role == "teacher":
        stmt = stmt.where(Class.teacher_id == current.id)
    classes = (await db.execute(stmt.order_by(Class.id))).scalars().all()
    return [
        {
            "id": c.id,
            "name": c.name,
            "course_id": c.course_id,
            "teacher_id": c.teacher_id,
            "student_count": c.student_count,
            "is_archived": c.is_archived,
        }
        for c in classes
    ]


@router.get("/{class_id}/students")
async def list_class_students(
    class_id: int, db: DbSession, current: CurrentUser
) -> list[dict[str, object]]:
    cls = await db.get(Class, class_id)
    if not cls:
        raise ResourceNotFoundError("class not found")
    if current.role == "teacher" and cls.teacher_id != current.id:
        raise AuthorizationError("无权查看他人班级")

    memberships = await _membership_repo.list_students_of_class(db, class_id)
    students_data = []
    for m in memberships:
        u = await db.get(User, m.student_id)
        if u is not None:
            students_data.append(
                {
                    "id": u.id,
                    "username": u.username,
                    "display_name": u.display_name,
                    "joined_at": m.joined_at.isoformat()
                    if m.joined_at
                    else None,
                }
            )
    return students_data


class CreateClassRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=64)
    course_id: int
    teacher_id: int | None = None


class BulkAddStudentsRequest(BaseModel):
    student_ids: list[int] = Field(..., min_length=1, max_length=200)


@router.post("", status_code=201)
async def create_class(
    payload: CreateClassRequest, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    cls = await org_service.create_class(
        db,
        actor=current,
        name=payload.name,
        course_id=payload.course_id,
        teacher_id=payload.teacher_id,
    )
    return {
        "id": cls.id,
        "name": cls.name,
        "course_id": cls.course_id,
        "teacher_id": cls.teacher_id,
    }


@router.patch("/{class_id}/archive")
async def archive_class(
    class_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    cls = await org_service.archive_class(db, actor=current, class_id=class_id)
    return {"id": cls.id, "is_archived": cls.is_archived}


@router.post("/{class_id}/students/bulk")
async def bulk_add_students(
    class_id: int,
    payload: BulkAddStudentsRequest,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    result = await org_service.bulk_add_students(
        db, actor=current, class_id=class_id, student_ids=payload.student_ids
    )
    return {
        "added": result.added,
        "failed": [{"student_id": sid, "reason": reason} for sid, reason in result.failed],
    }
