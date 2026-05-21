"""密码哈希 + JWT 签发与解析."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta
from typing import Any

import bcrypt
from jose import JWTError, jwt

from app.core.config import get_settings
from app.core.exceptions import AuthenticationError


def hash_password(plain: str) -> str:
    pwd = plain.encode("utf-8")[:72]  # bcrypt 限制 72 字节
    return bcrypt.hashpw(pwd, bcrypt.gensalt(rounds=12)).decode("utf-8")


def verify_password(plain: str, hashed: str) -> bool:
    pwd = plain.encode("utf-8")[:72]
    try:
        return bcrypt.checkpw(pwd, hashed.encode("utf-8"))
    except ValueError:
        return False


def create_access_token(
    *, user_id: int, role: str, expires_delta: timedelta | None = None
) -> str:
    s = get_settings()
    now = datetime.now(UTC)
    payload: dict[str, Any] = {
        "sub": str(user_id),
        "role": role,
        "iat": int(now.timestamp()),
        "exp": int(
            (now + (expires_delta or timedelta(minutes=s.jwt_access_ttl_minutes))).timestamp()
        ),
        "type": "access",
    }
    return jwt.encode(payload, s.jwt_secret, algorithm=s.jwt_algorithm)


def create_refresh_token(*, user_id: int) -> str:
    s = get_settings()
    now = datetime.now(UTC)
    payload: dict[str, Any] = {
        "sub": str(user_id),
        "iat": int(now.timestamp()),
        "exp": int((now + timedelta(days=s.jwt_refresh_ttl_days)).timestamp()),
        "type": "refresh",
    }
    return jwt.encode(payload, s.jwt_secret, algorithm=s.jwt_algorithm)


def decode_token(token: str) -> dict[str, Any]:
    s = get_settings()
    try:
        return jwt.decode(token, s.jwt_secret, algorithms=[s.jwt_algorithm])
    except JWTError as e:
        raise AuthenticationError(f"invalid token: {e}") from e
