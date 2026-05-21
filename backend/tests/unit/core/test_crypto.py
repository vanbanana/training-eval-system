"""Task 1.4 验收：AES-256-GCM 加密工具."""

from __future__ import annotations

import base64
import secrets

import pytest
from cryptography.exceptions import InvalidTag

from app.core.crypto import decrypt, derive_master_key, encrypt


@pytest.fixture()
def key32() -> bytes:
    """每次测试新生成 32 字节随机密钥."""
    return secrets.token_bytes(32)


class TestEncryptDecryptHappyPath:
    def test_encrypt_decrypt_roundtrip(self, key32: bytes) -> None:
        """Given 32 字节密钥与明文；When encrypt -> decrypt；Then 还原原文。"""
        plaintext = "sk-1234567890"
        cipher = encrypt(plaintext, key32)

        assert cipher != plaintext
        decoded = decrypt(cipher, key32)
        assert decoded == plaintext

    def test_cipher_is_base64(self, key32: bytes) -> None:
        cipher = encrypt("hello", key32)
        # base64 解码不抛异常
        raw = base64.b64decode(cipher)
        # 长度 = nonce(12) + plaintext(5) + tag(16) = 33
        assert len(raw) >= 12 + 5 + 16

    def test_unicode_roundtrip(self, key32: bytes) -> None:
        original = "你好，世界 🚀 sk-中文密钥"
        cipher = encrypt(original, key32)
        assert decrypt(cipher, key32) == original


class TestEncryptionFailures:
    def test_wrong_key_raises_invalid_tag(self) -> None:
        """Given 用 key_a 加密、用 key_b 解密；When decrypt；Then InvalidTag."""
        key_a = secrets.token_bytes(32)
        key_b = secrets.token_bytes(32)

        cipher = encrypt("secret", key_a)

        with pytest.raises(InvalidTag):
            decrypt(cipher, key_b)

    def test_short_key_rejected_on_encrypt(self) -> None:
        """Given key 长度 16；When encrypt；Then ValueError 含长度提示。"""
        with pytest.raises(ValueError, match="32 bytes"):
            encrypt("data", b"\x00" * 16)

    def test_short_key_rejected_on_decrypt(self, key32: bytes) -> None:
        cipher = encrypt("data", key32)
        with pytest.raises(ValueError, match="32 bytes"):
            decrypt(cipher, b"\x00" * 16)

    def test_corrupt_ciphertext_raises(self, key32: bytes) -> None:
        cipher = encrypt("data", key32)
        # 翻转最后 4 个字符（tag 区域）
        corrupted = cipher[:-4] + ("A" if cipher[-4:] != "AAAA" else "B") * 4
        with pytest.raises((InvalidTag, ValueError)):
            decrypt(corrupted, key32)

    def test_too_short_ciphertext_raises(self, key32: bytes) -> None:
        too_short = base64.b64encode(b"\x00" * 5).decode()
        with pytest.raises(ValueError, match="too short"):
            decrypt(too_short, key32)


class TestNonceRandomness:
    def test_nonce_uniqueness_in_1000_calls(self, key32: bytes) -> None:
        """Given 同一 plaintext 加密 1000 次；When 收集 cipher；Then 全部互不相同。"""
        plaintext = "fixed-input"
        ciphers = {encrypt(plaintext, key32) for _ in range(1000)}
        assert len(ciphers) == 1000


class TestDeriveMasterKey:
    def test_long_b64_key_truncated(self) -> None:
        """Given base64 解码后 ≥ 32 字节的字符串；When derive；Then 取前 32 字节。"""
        # 48 字节随机 base64
        long_raw = secrets.token_bytes(48)
        b64 = base64.b64encode(long_raw).decode()
        derived = derive_master_key(b64)
        assert len(derived) == 32
        assert derived == long_raw[:32]

    def test_short_b64_key_padded_via_sha256(self) -> None:
        """Given 解码后 < 32 字节；When derive；Then sha256(原 b64 字符串)。"""
        short = "short"
        derived = derive_master_key(short)
        assert len(derived) == 32

    def test_dev_default_master_key_works(self) -> None:
        """Given 项目 dev 默认值；When derive；Then 32 字节有效密钥。"""
        from app.core.config import Settings

        # 不依赖环境变量，使用类默认值
        s = Settings(jwt_secret="x" * 32)  # type: ignore[call-arg]
        key = derive_master_key(s.llm_key_master)
        assert len(key) == 32

        # 用此密钥可正常 encrypt/decrypt
        cipher = encrypt("test", key)
        assert decrypt(cipher, key) == "test"
