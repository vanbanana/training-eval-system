"""按 file_type 路由到对应 Parser."""

from __future__ import annotations

from app.parsers.base import Parser, ParserUnsupportedTypeError
from app.parsers.code_archive_parser import CodeArchiveParser
from app.parsers.docx_parser import DocxParser
from app.parsers.excel_parser import ExcelParser
from app.parsers.ocr_parser import ImageOcrParser
from app.parsers.pdf_parser import PdfParser


_REGISTRY: dict[str, type[Parser]] = {
    # 文档类
    "docx": DocxParser,
    "doc": DocxParser,  # .doc 旧格式尝试用 docx 解析（部分兼容）
    "pdf": PdfParser,
    # 表格类
    "xlsx": ExcelParser,
    # 图片类（OCR）
    "png": ImageOcrParser,
    "jpg": ImageOcrParser,
    "jpeg": ImageOcrParser,
    # 源代码压缩包
    "zip": CodeArchiveParser,
}

# 所有支持的文件扩展名（用于前端提示和上传校验）
SUPPORTED_EXTENSIONS: set[str] = set(_REGISTRY.keys())


def get_parser(file_type: str) -> Parser:
    """按文件类型获取对应 parser；不支持抛异常."""
    norm = file_type.lower().lstrip(".")
    if norm not in _REGISTRY:
        supported = ", ".join(sorted(SUPPORTED_EXTENSIONS))
        raise ParserUnsupportedTypeError(
            f"暂不支持的文件类型 '{file_type}'。当前支持: {supported}",
            field="file_type",
        )
    return _REGISTRY[norm]()  # type: ignore[abstract]
