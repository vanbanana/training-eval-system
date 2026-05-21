"""文档解析路由（mock 版 - 正式版用 Tesseract + python-docx + PyPDF2）."""

from __future__ import annotations

from fastapi import APIRouter

from app.api.deps import CurrentUser, DbSession
from app.core.exceptions import ResourceNotFoundError
from app.models.upload import Upload

router = APIRouter(prefix="/api/parse", tags=["parse"])


@router.post("/{upload_id}/trigger")
async def trigger_parse(upload_id: int, db: DbSession, current: CurrentUser) -> dict[str, object]:
    """触发文档解析（mock：直接标记为 parsed）."""
    upload = await db.get(Upload, upload_id)
    if not upload:
        raise ResourceNotFoundError("upload not found")
    upload.parse_status = "parsed"
    await db.commit()
    return {"upload_id": upload_id, "parse_status": "parsed", "extracted_text_preview": "（mock）文档内容已提取，共约 2000 字。"}


@router.get("/{upload_id}/result")
async def get_parse_result(upload_id: int, db: DbSession, current: CurrentUser) -> dict[str, object]:
    """获取解析结果."""
    upload = await db.get(Upload, upload_id)
    if not upload:
        raise ResourceNotFoundError("upload not found")
    return {
        "upload_id": upload_id,
        "parse_status": upload.parse_status,
        "filename": upload.filename,
        "file_type": upload.file_type,
        "extracted_text": "（mock）这是从文档中提取的文本内容。正式版将使用 Tesseract OCR / python-docx / PyPDF2 提取。",
    }
