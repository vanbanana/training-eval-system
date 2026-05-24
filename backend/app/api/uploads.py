"""学生上传路由 - 调用 UploadService."""

from __future__ import annotations

from fastapi import APIRouter, HTTPException, UploadFile

from app.api.deps import CurrentUser, DbSession
from app.core.config import get_settings
from app.core.exceptions import AuthorizationError
from app.repositories.upload_repo import UploadRepository
from app.schemas.upload import UploadOut
from app.services.upload_service import UploadInput, UploadService
from app.storage import LocalFileStorage

router = APIRouter(prefix="/api/uploads", tags=["uploads"])


def _get_service() -> UploadService:
    storage = LocalFileStorage(get_settings().upload_root)
    return UploadService(storage)


@router.post("/{task_id}", response_model=UploadOut, status_code=201)
async def upload_file(
    task_id: int,
    file: UploadFile,
    db: DbSession,
    current: CurrentUser,
) -> UploadOut:
    if current.role != "student":
        raise AuthorizationError("仅学生可提交")
    content = await file.read()
    svc = _get_service()
    upload = await svc.create_upload(
        db,
        student=current,
        task_id=task_id,
        file=UploadInput(
            filename=file.filename or "unnamed",
            content=content,
        ),
    )
    # 上传成功后自动触发解析（异步 Celery 任务）
    try:
        from app.tasks.parse_tasks import parse_upload_task

        parse_upload_task.delay(upload.id)
    except Exception as e:  # noqa: BLE001
        # Celery 不可用不阻塞上传
        from app.core.logging import get_logger

        get_logger(__name__).warning(
            "upload.auto_parse_enqueue_failed", upload_id=upload.id, error=str(e)
        )
    return UploadOut.model_validate(upload, from_attributes=True)


@router.get("/{task_id}", response_model=list[UploadOut])
async def list_uploads(
    task_id: int, db: DbSession, current: CurrentUser
) -> list[UploadOut]:
    repo = UploadRepository()
    items = await repo.list_by_task_student(
        db, task_id=task_id, student_id=current.id
    )
    return [UploadOut.model_validate(u, from_attributes=True) for u in items]


@router.delete("/{upload_id}", status_code=204)
async def delete_upload(
    upload_id: int, db: DbSession, current: CurrentUser
) -> None:
    svc = _get_service()
    await svc.delete_upload(db, actor=current, upload_id=upload_id)


@router.post("/{upload_id}/reparse")
async def reparse_upload(
    upload_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    svc = _get_service()
    upload = await svc.reparse(db, actor=current, upload_id=upload_id)
    return {"id": upload.id, "parse_status": upload.parse_status}


@router.get("/{upload_id}/parse-status")
async def parse_status(
    upload_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    repo = UploadRepository()
    upload = await repo.get(db, upload_id)
    if upload is None:
        raise HTTPException(status_code=404, detail="upload not found")
    if (
        current.role == "student"
        and upload.student_id != current.id
    ):
        raise AuthorizationError("无权查看")
    return {
        "upload_id": upload.id,
        "parse_status": upload.parse_status,
        "version": upload.version,
    }


@router.get("/{upload_id}/verify-result")
async def get_verify_result(
    upload_id: int, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    """Epic 15.4: 教师可读，学生只可读自己的；未完成核查 → 404 VERIFY_NOT_READY."""
    from sqlalchemy import select
    from sqlalchemy.orm import selectinload

    from app.core.exceptions import ResourceNotFoundError
    from app.models.upload import Upload

    upload = (
        await db.execute(
            select(Upload)
            .options(selectinload(Upload.verify_result))
            .where(Upload.id == upload_id)
        )
    ).scalar_one_or_none()
    if upload is None:
        raise ResourceNotFoundError(f"upload {upload_id} not found")
    if current.role == "student" and upload.student_id != current.id:
        raise AuthorizationError("无权查看")
    vr = upload.verify_result
    if vr is None:
        raise ResourceNotFoundError(
            "核查尚未完成", field="verify_result"
        )
    return {
        "upload_id": upload_id,
        "match_rate": float(vr.match_rate or 0),
        "checkpoints": vr.checkpoints or [],
        "missing_items": vr.missing_items or [],
        "logic_issues": vr.logic_issues or [],
        "overall_confidence": vr.overall_confidence or 0,
    }
