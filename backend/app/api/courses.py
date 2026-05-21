"""课程管理路由."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import ResourceNotFoundError
from app.models.course import Class, Course
from app.services.org_service import org_service

router = APIRouter(prefix="/api/courses", tags=["courses"])


class CreateCourseRequest(BaseModel):
    name: str = Field(..., min_length=1, max_length=100)
    code: str = Field(..., min_length=2, max_length=32)


class CourseOut(BaseModel):
    id: int
    name: str
    code: str
    is_archived: bool
    class_count: int


@router.get("", response_model=list[CourseOut])
async def list_courses(
    db: DbSession,
    current: CurrentUser,
) -> list[CourseOut]:
    courses = (await db.execute(select(Course).order_by(Course.id))).scalars().all()
    return [
        CourseOut(
            id=c.id,
            name=c.name,
            code=c.code,
            is_archived=c.is_archived,
            class_count=len(c.classes),
        )
        for c in courses
    ]


@router.post("", status_code=201)
async def create_course(
    payload: CreateCourseRequest, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    course = await org_service.create_course(
        db, actor=current, name=payload.name, code=payload.code
    )
    return {
        "id": course.id,
        "name": course.name,
        "code": course.code,
        "is_archived": course.is_archived,
    }


@router.patch("/{course_id}/archive")
async def archive_course(
    course_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    course = await org_service.archive_course(db, actor=current, course_id=course_id)
    return {"id": course.id, "is_archived": course.is_archived}


@router.get("/{course_id}/classes")
async def list_classes(
    course_id: int,
    db: DbSession,
    current: CurrentUser,
) -> list[dict[str, object]]:
    course = await db.get(Course, course_id)
    if not course:
        raise ResourceNotFoundError("course not found")
    classes = (
        await db.execute(select(Class).where(Class.course_id == course_id))
    ).scalars().all()
    return [
        {
            "id": c.id,
            "name": c.name,
            "teacher_id": c.teacher_id,
            "student_count": c.student_count,
            "is_archived": c.is_archived,
        }
        for c in classes
    ]
