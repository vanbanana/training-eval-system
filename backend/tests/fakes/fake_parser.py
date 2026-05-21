"""FakeOcr / FakeParser - 测试替身."""

from __future__ import annotations

from app.parsers.base import ParsedDocument


class FakeParser:
    """可预设输出的 parser；便于上层测试不依赖真实文件解析."""

    def __init__(
        self,
        file_type: str = "docx",
        output: ParsedDocument | None = None,
    ) -> None:
        self.file_type = file_type
        self._output = output or ParsedDocument(raw_text="fake content")

    async def parse(self, content: bytes) -> ParsedDocument:
        return self._output


class FakeOcrParser:
    file_type = "png"

    def __init__(self, text: str = "fake OCR text") -> None:
        self._text = text

    async def parse(self, content: bytes) -> ParsedDocument:
        return ParsedDocument(
            raw_text=self._text, paragraphs=[self._text], page_count=1
        )
