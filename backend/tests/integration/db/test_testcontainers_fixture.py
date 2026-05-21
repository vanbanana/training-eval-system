"""Task 2.5 验收：testcontainers fixture 骨架.

注意：本文件不强制启动真实 PostgreSQL；
仅验证 fixture 定义存在 + 跳过逻辑正确。
"""

from __future__ import annotations

import inspect
from pathlib import Path

import pytest


def test_conftest_exposes_pg_url_fixture() -> None:
    """Given conftest.py；When 检查；Then 注册 pg_url session-scoped fixture。"""
    from tests import conftest

    assert hasattr(conftest, "pg_url")
    sig = inspect.signature(conftest.pg_url)
    # 是 generator fixture
    assert "Generator" in str(conftest.pg_url.__annotations__.get("return", ""))


def test_conftest_exposes_redis_url_fixture() -> None:
    from tests import conftest

    assert hasattr(conftest, "redis_url")


def test_conftest_exposes_sqlite_session_fixture() -> None:
    from tests import conftest

    assert hasattr(conftest, "sqlite_session")


def test_conftest_skip_logic_documented() -> None:
    """Given conftest 源码；When 读；Then 含 SKIP_TESTCONTAINERS 环境变量分支。"""
    src = (Path(__file__).resolve().parent.parent.parent / "conftest.py").read_text(
        encoding="utf-8"
    )
    assert "SKIP_TESTCONTAINERS" in src


@pytest.mark.skipif(
    True, reason="本测试需 Docker 运行；CI 中开启即可"
)
def test_pg_fixture_can_be_used() -> None:  # pragma: no cover
    """占位测试：在 CI 启用 Docker 后取消 skipif 即可运行。"""
    pass


async def test_sqlite_session_fixture_works(sqlite_session: object) -> None:
    """Given sqlite_session fixture；When 注入；Then 拿到 AsyncSession 实例。"""
    from sqlalchemy.ext.asyncio import AsyncSession

    assert isinstance(sqlite_session, AsyncSession)
