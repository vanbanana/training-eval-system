"""文档解析路由 - 触发解析、查询结果、重新解析.

解析流程：
- 上传完成后自动触发（通过 Celery task）
- 教师/学生可手动触发重新解析
- 解析进度通过 WebSocket 推送
"""

from __future__ import annotations

from fastapi import APIRouter

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.models.upload import ParseResult, Upload
from app.schemas.parse import ParseResultOut, ParseStatusOut, ParseTriggerOut

router = APIRouter(prefix="/api/parse", tags=["parse"])
log = get_logger(__name__)


@router.get("/supported-formats")
async def get_supported_formats() -> dict[str, object]:
    """返回系统支持的文件格式列表（前端用于上传提示）."""
    from app.core.config import get_settings

    settings = get_settings()
    formats = [
        {
            "extension": "docx",
            "description": "Word 文档（.docx）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "document",
        },
        {
            "extension": "doc",
            "description": "Word 文档（.doc 旧格式）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "document",
        },
        {
            "extension": "pdf",
            "description": "PDF 文档（含扫描版自动 OCR）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "document",
        },
        {
            "extension": "xlsx",
            "description": "Excel 表格（.xlsx）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "spreadsheet",
        },
        {
            "extension": "png",
            "description": "PNG 图片（OCR 识别）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "image",
        },
        {
            "extension": "jpg",
            "description": "JPEG 图片（OCR 识别）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "image",
        },
        {
            "extension": "jpeg",
            "description": "JPEG 图片（OCR 识别）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "image",
        },
        {
            "extension": "zip",
            "description": "源代码压缩包（.zip，自动解压分析代码结构）",
            "max_size_mb": settings.max_upload_size_mb,
            "category": "archive",
        },
    ]
    return {"formats": formats}


@router.post("/{upload_id}/trigger", response_model=ParseTriggerOut)
async def trigger_parse(
    upload_id: int, db: DbSession, current: CurrentUser
) -> ParseTriggerOut:
    """触发文档解析（异步 Celery 任务）.

    权限：
    - 学生只能触发自己的上传
    - 教师可触发任意上传（用于重新解析）
    - 管理员可触发任意上传
    """
    upload = await db.get(Upload, upload_id)
    if not upload or upload.is_deleted:
        raise ResourceNotFoundError("upload not found")

    # 权限检查
    if current.role == "student" and upload.student_id != current.id:
        raise AuthorizationError("仅可触发自己的上传解析")

    # 状态检查：只有 pending 或 failed 状态可以触发
    if upload.parse_status not in {"pending", "failed"}:
        if current.role == "student":
            raise BusinessRuleError(
                f"当前状态 {upload.parse_status}，无法重新触发解析",
                field="parse_status",
            )
        # 教师/管理员可以强制重新解析
        upload.parse_status = "pending"
        await db.flush()

    # 发送 Celery 任务
    try:
        from app.tasks.parse_tasks import parse_upload_task

        parse_upload_task.delay(upload_id)
        log.info(
            "parse.trigger.enqueued",
            upload_id=upload_id,
            actor_id=current.id,
        )
    except Exception as e:  # noqa: BLE001
        # Celery 不可用时降级为同步解析
        log.warning("parse.trigger.celery_unavailable", error=str(e))
        await _fallback_sync_parse(db, upload_id)

    await db.commit()
    return ParseTriggerOut(
        upload_id=upload_id,
        parse_status=upload.parse_status,
        message="解析任务已提交，请通过 WebSocket 或轮询查看进度",
    )


@router.get("/{upload_id}/result", response_model=ParseResultOut)
async def get_parse_result(
    upload_id: int, db: DbSession, current: CurrentUser
) -> ParseResultOut:
    """获取解析结果."""
    upload = await db.get(Upload, upload_id)
    if not upload or upload.is_deleted:
        raise ResourceNotFoundError("upload not found")

    # 权限检查
    if current.role == "student" and upload.student_id != current.id:
        raise AuthorizationError("无权查看")

    parse_result = upload.parse_result
    if parse_result is None:
        raise ResourceNotFoundError(
            "解析尚未完成", field="parse_result"
        )

    # 如果有错误信息，说明解析失败
    if parse_result.error_message:
        return ParseResultOut(
            upload_id=upload_id,
            parse_status=upload.parse_status,
            filename=upload.filename,
            file_type=upload.file_type,
            raw_text="",
            structured_content=None,
            error_message=parse_result.error_message,
            parsed_at=parse_result.parsed_at,
        )

    return ParseResultOut(
        upload_id=upload_id,
        parse_status=upload.parse_status,
        filename=upload.filename,
        file_type=upload.file_type,
        raw_text=parse_result.raw_text[:10000],  # 前端展示截断
        structured_content=parse_result.structured_content,
        error_message="",
        parsed_at=parse_result.parsed_at,
    )


@router.get("/{upload_id}/status", response_model=ParseStatusOut)
async def get_parse_status(
    upload_id: int, db: DbSession, current: CurrentUser
) -> ParseStatusOut:
    """轮询解析状态（WebSocket 不可用时的降级方案）."""
    upload = await db.get(Upload, upload_id)
    if not upload or upload.is_deleted:
        raise ResourceNotFoundError("upload not found")

    if current.role == "student" and upload.student_id != current.id:
        raise AuthorizationError("无权查看")

    return ParseStatusOut(
        upload_id=upload_id,
        parse_status=upload.parse_status,
        filename=upload.filename,
        file_type=upload.file_type,
    )


async def _fallback_sync_parse(db: DbSession, upload_id: int) -> None:
    """Celery 不可用时的同步降级解析（仅 dev 环境）."""
    from app.core.config import get_settings
    from app.llm.factory import llm_factory
    from app.services.parse_pipeline import ParsePipeline
    from app.storage import LocalFileStorage

    settings = get_settings()
    if settings.env == "prod":
        # 生产环境不做同步降级，直接标记失败
        upload = await db.get(Upload, upload_id)
        if upload:
            upload.parse_status = "failed"
        return

    storage = LocalFileStorage(settings.upload_root)
    llm = None
    try:
        llm = await llm_factory.current(db)
    except Exception:  # noqa: BLE001
        pass

    pipeline = ParsePipeline(storage=storage, llm=llm)
    await pipeline.run(db, upload_id=upload_id)
