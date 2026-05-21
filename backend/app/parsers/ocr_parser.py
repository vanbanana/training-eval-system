"""ImageOcrParser - 基于 pytesseract（占位实现）.

生产环境需安装系统依赖 tesseract-ocr，并支持中英文。
本占位实现：若未安装 pytesseract，返回空 ParsedDocument 不抛异常（便于 dev 测试）。
"""

from __future__ import annotations

from app.parsers.base import ParsedDocument


class ImageOcrParser:
    file_type = "png"

    def __init__(self, languages: str = "chi_sim+eng") -> None:
        self.languages = languages

    async def parse(self, content: bytes) -> ParsedDocument:
        try:
            import pytesseract  # type: ignore[import-not-found]
            from PIL import Image  # type: ignore[import-not-found]
        except ImportError:
            return ParsedDocument(raw_text="", page_count=0)

        try:
            import io

            image = Image.open(io.BytesIO(content))
            text = pytesseract.image_to_string(image, lang=self.languages)
        except Exception:  # noqa: BLE001
            return ParsedDocument(raw_text="", page_count=0)

        return ParsedDocument(
            raw_text=text.strip(),
            paragraphs=[p.strip() for p in text.split("\n") if p.strip()],
            page_count=1,
        )
