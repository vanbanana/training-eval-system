"""文件头 magic number 校验工具 - 纯 Python 实现，不依赖 libmagic.

支持识别：docx / doc / pdf / png / jpg / xlsx / zip
防呆检测：拒绝可执行文件（exe/sh/bat/js 等）。
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
    # ZIP / DOCX / XLSX / PPTX 都以 PK\x03\x04 开头（OOXML 是 zip）
    (b"PK\x03\x04", "zip"),
    # DOC（旧版 Word）: D0 CF 11 E0 A1 B1 1A E1
    (b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1", "doc"),
    # RAR: 52 61 72 21 1A 07
    (b"Rar!\x1a\x07", "rar"),
    # 7z: 37 7A BC AF 27 1C
    (b"7z\xbc\xaf'\x1c", "7z"),
    # EXE/DLL (MZ header) - 用于拒绝
    (b"MZ", "exe"),
    # ELF (Linux 可执行) - 用于拒绝
    (b"\x7fELF", "elf"),
]

# 允许上传的扩展名白名单
_ALLOWED_EXTENSIONS: set[str] = {
    "docx", "doc", "pdf", "png", "jpg", "jpeg", "xlsx", "zip",
}

# 明确拒绝的危险扩展名
_DANGEROUS_EXTENSIONS: set[str] = {
    "exe", "dll", "so", "dylib",
    "sh", "bash", "bat", "cmd", "ps1",
    "js", "vbs", "wsf", "msi", "com",
    "elf", "bin", "app",
    "7z", "rar", "tar", "gz", "bz2", "xz",  # 仅支持 zip
}


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

    防呆检测：
    1. 拒绝危险扩展名
    2. 拒绝不在白名单中的扩展名
    3. 校验 magic number 与扩展名一致性
    """
    ext = filename.rsplit(".", 1)[-1].lower() if "." in filename else ""

    # 防呆：拒绝危险文件
    if ext in _DANGEROUS_EXTENSIONS:
        raise BusinessRuleError(
            f"不允许上传 .{ext} 类型文件（安全限制）",
            field="file",
        )

    # 防呆：白名单检查
    if ext not in _ALLOWED_EXTENSIONS:
        allowed = ", ".join(f".{e}" for e in sorted(_ALLOWED_EXTENSIONS))
        raise BusinessRuleError(
            f"不支持的文件类型 .{ext}。当前支持: {allowed}",
            field="file",
        )

    detected = detect_file_type(content_head)

    # 无法识别 magic（可能是纯文本或损坏文件）
    if detected is None:
        raise BusinessRuleError(
            "无法识别文件类型（文件可能为空或已损坏）", field="file"
        )

    # 拒绝可执行文件（即使扩展名伪装）
    if detected in {"exe", "elf"}:
        raise BusinessRuleError(
            "检测到可执行文件内容，拒绝上传", field="file"
        )

    # docx/xlsx/zip 的 magic 都是 "zip"（PK header），按扩展名区分
    if detected == "zip" and ext in {"docx", "xlsx", "zip"}:
        return ext

    # doc 旧格式
    if detected == "doc" and ext == "doc":
        return ext

    # jpeg = jpg
    if detected == "jpg" and ext in {"jpg", "jpeg"}:
        return ext

    # 精确匹配
    if detected == ext:
        return ext

    raise BusinessRuleError(
        f"文件扩展名 .{ext} 与实际文件内容类型 ({detected}) 不一致",
        field="file",
    )
