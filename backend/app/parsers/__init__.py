"""文档解析器层.

支持的文件类型：
- docx/doc: Word 文档（python-docx）
- pdf: PDF 文档（pypdf，扫描版自动 OCR 降级）
- xlsx: Excel 表格（openpyxl）
- png/jpg/jpeg: 图片 OCR（pytesseract）
- zip: 源代码压缩包
"""

from app.parsers.base import (
    ParsedDocument,
    Parser,
    ParserError,
    ParserUnsupportedTypeError,
)
from app.parsers.factory import SUPPORTED_EXTENSIONS, get_parser

__all__ = [
    "ParsedDocument",
    "Parser",
    "ParserError",
    "ParserUnsupportedTypeError",
    "SUPPORTED_EXTENSIONS",
    "get_parser",
]
