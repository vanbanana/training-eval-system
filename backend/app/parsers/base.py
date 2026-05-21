"""Parser Protocol 与统一输出 schema."""

from __future__ import annotations

from typing import Protocol, runtime_checkable

from pydantic import BaseModel, Field

from app.core.exceptions import BusinessRuleError


class ParserError(BusinessRuleError):
    error_code = "PARSER_ERROR"


class ParserUnsupportedTypeError(ParserError):
    error_code = "PARSER_UNSUPPORTED_TYPE"


class TitleNode(BaseModel):
    """标题树节点（支持 6 级嵌套）."""

    level: int = Field(..., ge=1, le=6)
    text: str
    children: list[TitleNode] = Field(default_factory=list)


class ParsedDocument(BaseModel):
    """所有 parser 输出的统一结构."""

    raw_text: str
    paragraphs: list[str] = Field(default_factory=list)
    title_tree: list[TitleNode] = Field(default_factory=list)
    images: list[str] = Field(default_factory=list)  # base64 或 path 引用
    tables: list[list[list[str]]] = Field(default_factory=list)  # 表格：行 x 列
    metadata: dict[str, str] = Field(default_factory=dict)
    page_count: int = 0


@runtime_checkable
class Parser(Protocol):
    """统一文档解析器接口."""

    file_type: str

    async def parse(self, content: bytes) -> ParsedDocument: ...
