"""上传相关 schemas."""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel


class UploadOut(BaseModel):
    id: int
    task_id: int
    student_id: int
    filename: str
    file_type: str
    file_size: int
    sha256: str
    parse_status: str
    version: int
    created_at: datetime


class ParsedDocumentOut(BaseModel):
    upload_id: int
    raw_text: str
    structured_content: dict | None = None
    parsed_at: datetime | None = None
