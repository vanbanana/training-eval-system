"""AES-256-GCM 加密工具.

用于加密存储 LLM API Key 等敏感字段。

格式：base64(nonce || ciphertext || tag)
- nonce: 12 字节随机
- tag: 16 字节认证标签

主密钥从 Settings.llm_key_master 注入（base64 编码的 32 字节）。
"""

from __future__ import annotations

import base64
import secrets

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

_NONCE_LEN = 12  # GCM 推荐
_KEY_LEN = 32  # AES-256


def _validate_key(key: bytes) -> None:
    if len(key) != _KEY_LEN:
        raise ValueError(f"key must be {_KEY_LEN} bytes (got {len(key)})")


def encrypt(plaintext: str, master_key: bytes) -> str:
    """加密；返回 base64(nonce || ct || tag)."""
    _validate_key(master_key)
    nonce = secrets.token_bytes(_NONCE_LEN)
    aesgcm = AESGCM(master_key)
    ct_with_tag = aesgcm.encrypt(nonce, plaintext.encode("utf-8"), associated_data=None)
    return base64.b64encode(nonce + ct_with_tag).decode("ascii")


def decrypt(ciphertext_b64: str, master_key: bytes) -> str:
    """解密 base64 输入；密钥不匹配抛 InvalidTag."""
    _validate_key(master_key)
    raw = base64.b64decode(ciphertext_b64)
    if len(raw) < _NONCE_LEN + 16:
        raise ValueError("ciphertext too short")
    nonce = raw[:_NONCE_LEN]
    ct_with_tag = raw[_NONCE_LEN:]
    aesgcm = AESGCM(master_key)
    plain = aesgcm.decrypt(nonce, ct_with_tag, associated_data=None)
    return plain.decode("utf-8")


def derive_master_key(b64_key: str) -> bytes:
    """从配置（base64 字符串）派生 32 字节主密钥.

    若解码后不足 32 字节，用 sha256 填充至 32 字节。
    若超过则截断。这允许 Settings 接受任意长度 ≥ 32 字符的 base64 字符串。
    """
    try:
        raw = base64.b64decode(b64_key, validate=False)
    except Exception:
        raw = b64_key.encode("utf-8")

    if len(raw) >= _KEY_LEN:
        return raw[:_KEY_LEN]

    # 不足 32 字节：用 sha256 派生
    import hashlib
    return hashlib.sha256(b64_key.encode("utf-8")).digest()
