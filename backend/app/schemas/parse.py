"""解析相关 Pydantic schemas."""

from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, Field


class ParseTriggerOut(BaseModel):
    """触发解析的响应."""

    upload_id: int
    parse_status: str
    message: str = ""


class ParseStatusOut(BaseModel):
    """解析状态轮询响应."""

    upload_id: int
    parse_status: str  # pending / parsing / parsed / failed
    filename: str
    file_type: str


class ParseResultOut(BaseModel):
    """解析结果详情."""

    upload_id: int
    parse_status: str
    filename: str
    file_type: str
    raw_text: str = ""
    structured_content: dict | None = None
    error_message: str = ""
    parsed_at: datetime | None = None


class FileFormatInfo(BaseModel):
    """单个文件格式信息."""

    extension: str = Field(..., description="文件扩展名（不含点）")
    description: str = Field(..., description="格式描述")
    max_size_mb: int = Field(..., description="最大文件大小（MB）")
    category: str = Field(..., description="分类: document/image/spreadsheet/archive")


class SupportedFormatsOut(BaseModel):
    """支持的文件格式列表."""

    formats: list[FileFormatInfo]
