"""PdfParser - 基于 PyMuPDF（fitz）.

LoongArch 注意：PyMuPDF 有 C 扩展，需确保发行版可编译。
若 import 失败，使用 pypdf 纯 Python 后备。
"""

from __future__ import annotations

import io

from app.parsers.base import ParsedDocument, ParserError


class PdfParser:
    file_type = "pdf"

    async def parse(self, content: bytes) -> ParsedDocument:
        # 优先使用 pypdf（纯 Python，LoongArch 兼容好）
        try:
            from pypdf import PdfReader
        except ImportError:
            try:
                from PyPDF2 import PdfReader  # type: ignore[import-not-found, no-redef]
            except ImportError as e:
                raise ParserError(
                    "需要 pypdf 或 PyPDF2 库", field="dependency"
                ) from e

        try:
            reader = PdfReader(io.BytesIO(content))
        except Exception as e:  # noqa: BLE001
            raise ParserError(f"pdf 解析失败: {e}", field="file") from e

        pages_text: list[str] = []
        for page in reader.pages:
            try:
                pages_text.append((page.extract_text() or "").strip())
            except Exception:  # noqa: BLE001
                pages_text.append("")

        raw_text = "\n\n".join(t for t in pages_text if t)
        paragraphs = [p.strip() for p in raw_text.split("\n\n") if p.strip()]

        return ParsedDocument(
            raw_text=raw_text,
            paragraphs=paragraphs,
            page_count=len(pages_text),
        )
