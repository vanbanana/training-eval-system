"""PdfParser - 基于 pypdf（纯 Python）.

LoongArch 兼容：pypdf 纯 Python，无 C 扩展。
扫描版 PDF 检测：如果提取文本过少（< 100 字符/页），自动降级为 OCR 模式。
"""

from __future__ import annotations

import io

from app.parsers.base import ParsedDocument, ParserError, TitleNode


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
        page_count = len(reader.pages)

        # 扫描版 PDF 检测：文本过少则尝试 OCR
        avg_chars_per_page = len(raw_text) / max(page_count, 1)
        if avg_chars_per_page < 100 and page_count > 0:
            # 尝试 OCR 降级
            ocr_text = await self._try_ocr(content)
            if ocr_text and len(ocr_text) > len(raw_text):
                raw_text = ocr_text

        paragraphs = [p.strip() for p in raw_text.split("\n\n") if p.strip()]

        # 尝试提取标题结构（基于字体大小或格式启发）
        title_tree = self._extract_titles(paragraphs)

        return ParsedDocument(
            raw_text=raw_text,
            paragraphs=paragraphs,
            title_tree=title_tree,
            page_count=page_count,
            metadata={
                "is_scanned": str(avg_chars_per_page < 100),
                "page_count": str(page_count),
            },
        )

    @staticmethod
    async def _try_ocr(content: bytes) -> str:
        """尝试对扫描版 PDF 进行 OCR（需要 pytesseract + pdf2image 或 fitz）."""
        try:
            import fitz  # PyMuPDF

            doc = fitz.open(stream=content, filetype="pdf")
            ocr_parts: list[str] = []
            for page_num in range(min(doc.page_count, 20)):  # 最多 OCR 20 页
                page = doc[page_num]
                # 尝试获取文本（PyMuPDF 有更好的文本提取）
                text = page.get_text("text").strip()
                if text:
                    ocr_parts.append(text)
            doc.close()
            return "\n\n".join(ocr_parts)
        except ImportError:
            pass
        except Exception:  # noqa: BLE001
            pass

        # 如果 PyMuPDF 不可用，尝试 pytesseract
        try:
            import pytesseract  # type: ignore[import-not-found]
            from pdf2image import convert_from_bytes  # type: ignore[import-not-found]

            images = convert_from_bytes(content, first_page=1, last_page=10)
            ocr_parts = []
            for img in images:
                text = pytesseract.image_to_string(img, lang="chi_sim+eng")
                if text.strip():
                    ocr_parts.append(text.strip())
            return "\n\n".join(ocr_parts)
        except ImportError:
            pass
        except Exception:  # noqa: BLE001
            pass

        return ""

    @staticmethod
    def _extract_titles(paragraphs: list[str]) -> list[TitleNode]:
        """启发式提取标题（基于段落格式特征）."""
        import re

        title_tree: list[TitleNode] = []
        # 匹配常见标题模式
        title_patterns = [
            (1, re.compile(r"^第[一二三四五六七八九十\d]+[章节篇]")),
            (2, re.compile(r"^[一二三四五六七八九十]+[、.]")),
            (2, re.compile(r"^\d+[.、]\s*\S")),
            (3, re.compile(r"^\d+\.\d+\s*\S")),
            (3, re.compile(r"^[（(]\d+[)）]")),
        ]

        for para in paragraphs:
            if len(para) > 100:  # 标题通常较短
                continue
            for level, pattern in title_patterns:
                if pattern.match(para):
                    title_tree.append(
                        TitleNode(level=level, text=para, children=[])
                    )
                    break

        return title_tree
