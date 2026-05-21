"""文件头 magic number 校验工具 - 纯 Python 实现，不依赖 libmagic.

支持识别：docx / doc / pdf / png / jpg / zip
"""

from __future__ import annotations

from app.core.exceptions import BusinessRuleError

# (magic bytes prefix, file_type) 对
_SIGNATURES: list[tuple[bytes, str]] = [
    # PDF: %PDF-
    (b"%PDF-", "pdf"),
    # PNG: 89 50 4E 47 0D 0A 1A 0A
    (b"\x89PNG\r\n\x1a\n", "png"),
    # JPG: FF D8 FF
    (b"\xff\xd8\xff", "jpg"),
    # ZIP / DOCX / XLSX 都以 PK\x03\x04 开头（OOXML 是 zip）
    # docx 由扩展名进一步识别
    (b"PK\x03\x04", "zip"),
    # DOC（旧版 Word）: D0 CF 11 E0 A1 B1 1A E1
    (b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1", "doc"),
]


def detect_file_type(head_bytes: bytes) -> str | None:
    """根据文件头 magic number 识别类型；空文件或无匹配返回 None."""
    if not head_bytes:
        return None
    for sig, ftype in _SIGNATURES:
        if head_bytes.startswith(sig):
            return ftype
    return None


def assert_extension_matches(filename: str, content_head: bytes) -> str:
    """断言文件扩展名与文件头一致；返回检测出的类型.

    docx/xlsx/zip 都会被识别为 "zip"，结合扩展名做精确判断。
    """
    detected = detect_file_type(content_head)
    if detected is None:
        raise BusinessRuleError(
            "无法识别文件类型（空文件或损坏）", field="file"
        )

    ext = filename.rsplit(".", 1)[-1].lower() if "." in filename else ""

    # docx/xlsx 的 magic 与 zip 一致，按扩展名通过
    if detected == "zip" and ext in {"docx", "xlsx", "pptx", "zip"}:
        return ext
    # jpeg = jpg
    if detected == "jpg" and ext in {"jpg", "jpeg"}:
        return ext
    if detected != ext:
        raise BusinessRuleError(
            f"文件扩展名 {ext} 与实际类型 {detected} 不一致",
            field="file",
        )
    return ext
