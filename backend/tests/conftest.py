"""全局 pytest 配置 - fixtures、event loop、testcontainers DB.

策略：
- 默认使用 in-memory SQLite（快、无外部依赖），适合单元/小型集成测试
- testcontainers 启动真实 PostgreSQL（标记 @pytest.mark.integration + 显式 fixture）
"""

from __future__ import annotations

import asyncio
import os
from collections.abc import AsyncIterator, Generator
from typing import Any

import pytest
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import (
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)


@pytest.fixture(scope="session")
def event_loop() -> Generator[asyncio.AbstractEventLoop, None, None]:
    """整个测试 session 共享一个 event loop（仅当显式请求此 fixture）."""
    loop = asyncio.new_event_loop()
    try:
        yield loop
    finally:
        loop.close()


@pytest.fixture()
async def http_client() -> AsyncIterator[AsyncClient]:
    """ASGI client，使用 raise_app_exceptions=False 让 500 也走异常处理器."""
    try:
        from app.main import app
    except ImportError:  # pragma: no cover
        pytest.skip("app.main 未就绪")

    transport = ASGITransport(app=app, raise_app_exceptions=False)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        yield client


@pytest.fixture()
async def sqlite_session() -> AsyncIterator[AsyncSession]:
    """单测试隔离的 in-memory SQLite session - 已建表，自动清理."""
    from app.core.database import Base

    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with SessionLocal() as s:
        yield s
    await engine.dispose()


# ============== testcontainers fixtures（按需启用）==============

@pytest.fixture(scope="session")
def pg_url() -> Generator[str, None, None]:
    """启动 testcontainers PostgreSQL；返回 async URL.

    跳过条件：
    - 环境变量 SKIP_TESTCONTAINERS=1
    - Docker 不可用
    - testcontainers 未安装

    使用：
        @pytest.mark.integration
        async def test_with_pg(pg_url: str) -> None:
            engine = create_async_engine(pg_url)
            ...
    """
    if os.getenv("SKIP_TESTCONTAINERS") == "1":
        pytest.skip("testcontainers disabled by env")

    try:
        from testcontainers.postgres import PostgresContainer
    except ImportError:
        pytest.skip("testcontainers not installed")

    try:
        with PostgresContainer("postgres:14-alpine") as pg:
            sync_url = pg.get_connection_url()
            # 转换为 asyncpg URL
            async_url = sync_url.replace("postgresql+psycopg2", "postgresql+asyncpg").replace(
                "postgresql://", "postgresql+asyncpg://"
            )
            yield async_url
    except Exception as e:  # pragma: no cover
        pytest.skip(f"无法启动 PostgresContainer: {e}")


@pytest.fixture(scope="session")
def redis_url() -> Generator[str, None, None]:
    """启动 testcontainers Redis；返回连接 URL."""
    if os.getenv("SKIP_TESTCONTAINERS") == "1":
        pytest.skip("testcontainers disabled by env")

    try:
        from testcontainers.redis import RedisContainer
    except ImportError:
        pytest.skip("testcontainers not installed")

    try:
        with RedisContainer("redis:7-alpine") as r:
            host = r.get_container_host_ip()
            port = r.get_exposed_port(6379)
            yield f"redis://{host}:{port}/0"
    except Exception as e:  # pragma: no cover
        pytest.skip(f"无法启动 RedisContainer: {e}")


@pytest.fixture()
def settings_override() -> Generator[dict[str, Any], None, None]:
    """临时覆盖 Settings - 写测试时通过 monkeypatch.setenv 调用更稳."""
    yield {}
