"""Imports API - Epic 25.4 / 25.5."""

from __future__ import annotations

from fastapi import APIRouter, UploadFile
from fastapi.responses import StreamingResponse

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import (
    AuthorizationError,
    ResourceNotFoundError,
)
from app.models.import_job import ImportJob
from app.services.import_service import (
    ImportService,
    export_class_students_xlsx,
    make_student_template_xlsx,
    make_user_template_xlsx,
    parse_student_xlsx,
    parse_user_xlsx,
)

router = APIRouter(prefix="/api/imports", tags=["imports"])


@router.get("/template/user.xlsx")
async def user_template(current: CurrentUser) -> StreamingResponse:
    if current.role not in ("admin", "teacher"):
        raise AuthorizationError("仅管理员/教师可下载模板")
    return StreamingResponse(
        iter([make_user_template_xlsx()]),
        media_type=(
            "application/vnd.openxmlformats-officedocument."
            "spreadsheetml.sheet"
        ),
        headers={
            "Content-Disposition": 'attachment; filename="user_template.xlsx"'
        },
    )


@router.get("/template/student.xlsx")
async def student_template(current: CurrentUser) -> StreamingResponse:
    if current.role not in ("admin", "teacher"):
        raise AuthorizationError("仅管理员/教师可下载模板")
    return StreamingResponse(
        iter([make_student_template_xlsx()]),
        media_type=(
            "application/vnd.openxmlformats-officedocument."
            "spreadsheetml.sheet"
        ),
        headers={
            "Content-Disposition": 'attachment; filename="student_template.xlsx"'
        },
    )


@router.post("/users", status_code=202)
async def import_users(
    file: UploadFile, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    if current.role != "admin":
        raise AuthorizationError("仅管理员可批量导入用户")
    content = await file.read()
    rows = parse_user_xlsx(content)
    svc = ImportService()
    job = await svc.start_user_import(db, operator=current, rows=rows)
    await db.commit()
    return {
        "job_id": job.id,
        "total": job.total_count,
        "success": job.success_count,
        "failed": job.failed_count,
        "status": job.status,
    }


@router.post("/classes/{class_id}/students", status_code=202)
async def import_class_students(
    class_id: int,
    file: UploadFile,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    if current.role not in ("admin", "teacher"):
        raise AuthorizationError("仅管理员/教师可导入")
    content = await file.read()
    usernames = parse_student_xlsx(content)
    svc = ImportService()
    job = await svc.start_class_student_import(
        db,
        operator=current,
        class_id=class_id,
        usernames=usernames,
    )
    await db.commit()
    return {
        "job_id": job.id,
        "total": job.total_count,
        "success": job.success_count,
        "failed": job.failed_count,
        "status": job.status,
    }


@router.get("/{job_id}")
async def get_job(
    job_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    job = await db.get(ImportJob, job_id)
    if job is None:
        raise ResourceNotFoundError(f"job {job_id} not found")
    if current.role != "admin" and job.operator_id != current.id:
        raise AuthorizationError("无权查看他人 job")
    return {
        "id": job.id,
        "operator_id": job.operator_id,
        "job_type": job.job_type,
        "status": job.status,
        "total": job.total_count,
        "success": job.success_count,
        "failed": job.failed_count,
        "completed_at": (
            job.completed_at.isoformat() if job.completed_at else None
        ),
    }


@router.get("/exports/class/{class_id}/students.xlsx")
async def export_class_students(
    class_id: int, db: DbSession, current: CurrentUser
) -> StreamingResponse:
    xlsx = await export_class_students_xlsx(
        db, class_id=class_id, operator=current
    )
    return StreamingResponse(
        iter([xlsx]),
        media_type=(
            "application/vnd.openxmlformats-officedocument."
            "spreadsheetml.sheet"
        ),
        headers={
            "Content-Disposition": (
                f'attachment; filename="class_{class_id}_students.xlsx"'
            )
        },
    )
