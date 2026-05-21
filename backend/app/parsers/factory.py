"""按 file_type 路由到对应 Parser."""

from __future__ import annotations

from app.parsers.base import Parser, ParserUnsupportedTypeError
from app.parsers.docx_parser import DocxParser
from app.parsers.ocr_parser import ImageOcrParser
from app.parsers.pdf_parser import PdfParser


_REGISTRY: dict[str, type[Parser]] = {
    "docx": DocxParser,
    "pdf": PdfParser,
    "png": ImageOcrParser,
    "jpg": ImageOcrParser,
    "jpeg": ImageOcrParser,
}


def get_parser(file_type: str) -> Parser:
    """按文件类型获取对应 parser；不支持抛异常."""
    norm = file_type.lower().lstrip(".")
    if norm not in _REGISTRY:
        raise ParserUnsupportedTypeError(
            f"暂不支持的文件类型 {file_type}", field="file_type"
        )
    return _REGISTRY[norm]()  # type: ignore[abstract]
