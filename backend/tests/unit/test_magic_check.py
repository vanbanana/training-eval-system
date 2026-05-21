"""Task 7.3 验收：文件头 magic number 校验工具."""

from __future__ import annotations

import pytest

from app.core.exceptions import BusinessRuleError
from app.utils.magic_check import assert_extension_matches, detect_file_type


pytestmark = pytest.mark.unit


class TestDetectFileType:
    def test_pdf_signature(self) -> None:
        assert detect_file_type(b"%PDF-1.4\nsome content") == "pdf"

    def test_png_signature(self) -> None:
        assert detect_file_type(b"\x89PNG\r\n\x1a\nIHDR...") == "png"

    def test_jpg_signature(self) -> None:
        assert detect_file_type(b"\xff\xd8\xff\xe0\x00\x10JFIF") == "jpg"

    def test_zip_signature(self) -> None:
        # docx, xlsx 都会被识别为 zip
        assert detect_file_type(b"PK\x03\x04...") == "zip"

    def test_doc_signature(self) -> None:
        assert detect_file_type(b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1xxx") == "doc"

    def test_empty_bytes_returns_none(self) -> None:
        assert detect_file_type(b"") is None

    def test_unknown_returns_none(self) -> None:
        assert detect_file_type(b"random bytes") is None


class TestAssertExtensionMatches:
    def test_pdf_extension_matches(self) -> None:
        result = assert_extension_matches("report.pdf", b"%PDF-1.4")
        assert result == "pdf"

    def test_docx_with_zip_magic_passes(self) -> None:
        # docx 是 zip 容器，magic 是 PK\x03\x04
        result = assert_extension_matches("report.docx", b"PK\x03\x04...")
        assert result == "docx"

    def test_jpeg_alias_for_jpg(self) -> None:
        result = assert_extension_matches("photo.jpeg", b"\xff\xd8\xff\xe0")
        assert result == "jpeg"

    def test_exe_renamed_to_pdf_rejected(self) -> None:
        """假装将 exe 改名为 pdf：magic 头 MZ 不匹配 pdf。"""
        with pytest.raises(BusinessRuleError) as exc:
            # MZ 头不在 _SIGNATURES 中，detected 为 None
            assert_extension_matches("malware.pdf", b"MZ\x90\x00...")
        assert "无法识别" in str(exc.value) or "不一致" in str(exc.value)

    def test_pdf_with_png_content_rejected(self) -> None:
        with pytest.raises(BusinessRuleError) as exc:
            assert_extension_matches("fake.pdf", b"\x89PNG\r\n\x1a\n")
        assert exc.value.field == "file"

    def test_empty_file_rejected(self) -> None:
        with pytest.raises(BusinessRuleError):
            assert_extension_matches("a.pdf", b"")
