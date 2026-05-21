"""Task 1.9 验收：JWT 编解码 + 密码哈希."""

from __future__ import annotations

from datetime import timedelta

import pytest
from freezegun import freeze_time

from app.core.config import get_settings
from app.core.exceptions import AuthenticationError
from app.core.security import (
    create_access_token,
    create_refresh_token,
    decode_token,
    hash_password,
    verify_password,
)


@pytest.fixture(autouse=True)
def _set_jwt_secret(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
    get_settings.cache_clear()


class TestPasswordHashing:
    def test_hash_then_verify_true(self) -> None:
        """Given 明文密码；When hash → verify；Then True."""
        hashed = hash_password("MyPassw0rd!")
        assert hashed != "MyPassw0rd!"
        assert verify_password("MyPassw0rd!", hashed) is True

    def test_verify_wrong_password_false(self) -> None:
        hashed = hash_password("correct")
        assert verify_password("wrong", hashed) is False

    def test_hash_different_each_time_with_salt(self) -> None:
        h1 = hash_password("same")
        h2 = hash_password("same")
        assert h1 != h2  # 盐随机
        assert verify_password("same", h1)
        assert verify_password("same", h2)

    def test_long_password_truncated_safely(self) -> None:
        """Given 超过 72 字节的密码；When hash；Then 不抛异常（bcrypt 限制内）."""
        long = "a" * 100
        hashed = hash_password(long)
        # 前 72 字节相同的密码视为匹配
        assert verify_password("a" * 80, hashed)  # 前 72 字节都是 'a'

    def test_verify_invalid_hash_returns_false_not_raise(self) -> None:
        assert verify_password("any", "not-a-valid-hash") is False


class TestAccessToken:
    def test_create_then_decode_returns_payload(self) -> None:
        """Given user_id=42 + role=teacher；When create+decode；
        Then sub==42、role==teacher、type==access。"""
        token = create_access_token(user_id=42, role="teacher")
        payload = decode_token(token)
        assert payload["sub"] == "42"
        assert payload["role"] == "teacher"
        assert payload["type"] == "access"
        assert "exp" in payload

    @freeze_time("2026-05-19 12:00:00")
    def test_default_expiry_60_minutes(self) -> None:
        token = create_access_token(user_id=1, role="student")
        payload = decode_token(token)
        # exp - iat = 60 * 60
        assert payload["exp"] - payload["iat"] == 60 * 60

    def test_custom_expires_delta(self) -> None:
        token = create_access_token(
            user_id=1, role="student", expires_delta=timedelta(seconds=10)
        )
        payload = decode_token(token)
        assert payload["exp"] - payload["iat"] == 10


class TestRefreshToken:
    def test_create_refresh_token_has_correct_type(self) -> None:
        token = create_refresh_token(user_id=99)
        payload = decode_token(token)
        assert payload["type"] == "refresh"
        assert payload["sub"] == "99"

    @freeze_time("2026-05-19 12:00:00")
    def test_refresh_token_default_expiry_7_days(self) -> None:
        token = create_refresh_token(user_id=1)
        payload = decode_token(token)
        assert payload["exp"] - payload["iat"] == 7 * 24 * 60 * 60


class TestDecodeFailures:
    def test_invalid_signature_raises_authentication_error(self) -> None:
        token = create_access_token(user_id=1, role="student")
        # 篡改最后 5 个字符
        tampered = token[:-5] + "AAAAA"
        with pytest.raises(AuthenticationError) as exc_info:
            decode_token(tampered)
        assert "invalid token" in str(exc_info.value).lower()

    @freeze_time("2026-05-19 12:00:00")
    def test_expired_token_raises(self, monkeypatch: pytest.MonkeyPatch) -> None:
        token = create_access_token(
            user_id=1, role="student", expires_delta=timedelta(seconds=1)
        )

        with freeze_time("2026-05-19 12:01:00"):  # 60s 后
            with pytest.raises(AuthenticationError):
                decode_token(token)

    def test_malformed_token_raises(self) -> None:
        with pytest.raises(AuthenticationError):
            decode_token("not.a.valid.jwt")
