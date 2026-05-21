"""Task 8.4 验收：断点续传."""

from __future__ import annotations

import hashlib
from collections.abc import AsyncIterator

import pytest
from fakeredis.aioredis import FakeRedis

from app.services.chunked_upload import (
    ChecksumMismatchError,
    append_chunk,
    assemble,
    cleanup_session,
    init_session,
    received_bytes,
)


pytestmark = pytest.mark.unit


@pytest.fixture()
async def redis() -> AsyncIterator[FakeRedis]:
    r = FakeRedis(decode_responses=False)
    yield r
    await r.aclose()


class TestSessionLifecycle:
    async def test_init_returns_session_id(self, redis: FakeRedis) -> None:
        sid = await init_session(
            redis,
            task_id=1,
            student_id=2,
            total_size=100,
            declared_sha256="x" * 64,
            filename="report.pdf",
        )
        assert isinstance(sid, str)
        assert len(sid) == 32

    async def test_append_chunks_in_order_and_assemble(
        self, redis: FakeRedis
    ) -> None:
        data = b"the quick brown fox jumps over the lazy dog" * 10
        sha = hashlib.sha256(data).hexdigest()
        sid = await init_session(
            redis,
            task_id=1,
            student_id=2,
            total_size=len(data),
            declared_sha256=sha,
            filename="x.pdf",
        )

        # 分 5 片
        chunk_size = len(data) // 5
        for i in range(5):
            start = i * chunk_size
            end = start + chunk_size if i < 4 else len(data)
            await append_chunk(
                redis, session_id=sid, start=start, chunk=data[start:end]
            )

        full, _ = await assemble(redis, session_id=sid)
        assert full == data

    async def test_resend_chunk_does_not_corrupt(
        self, redis: FakeRedis
    ) -> None:
        """Given 中间一片重发；When assemble；Then 结果不变。"""
        data = b"abc" * 100
        sha = hashlib.sha256(data).hexdigest()
        sid = await init_session(
            redis,
            task_id=1,
            student_id=2,
            total_size=len(data),
            declared_sha256=sha,
            filename="x.pdf",
        )

        # 分 3 片
        await append_chunk(redis, session_id=sid, start=0, chunk=data[0:100])
        await append_chunk(redis, session_id=sid, start=100, chunk=data[100:200])
        # 重发第二片
        await append_chunk(redis, session_id=sid, start=100, chunk=data[100:200])
        await append_chunk(redis, session_id=sid, start=200, chunk=data[200:])

        full, _ = await assemble(redis, session_id=sid)
        assert full == data


class TestChecksum:
    async def test_mismatched_sha_raises(self, redis: FakeRedis) -> None:
        data = b"x" * 100
        wrong_sha = "0" * 64
        sid = await init_session(
            redis,
            task_id=1,
            student_id=2,
            total_size=len(data),
            declared_sha256=wrong_sha,
            filename="x.pdf",
        )
        await append_chunk(redis, session_id=sid, start=0, chunk=data)

        with pytest.raises(ChecksumMismatchError):
            await assemble(redis, session_id=sid)


class TestReceivedBytes:
    async def test_returns_running_total(self, redis: FakeRedis) -> None:
        data = b"x" * 1000
        sha = hashlib.sha256(data).hexdigest()
        sid = await init_session(
            redis,
            task_id=1,
            student_id=2,
            total_size=1000,
            declared_sha256=sha,
            filename="x.pdf",
        )
        assert await received_bytes(redis, sid) == 0
        await append_chunk(redis, session_id=sid, start=0, chunk=data[:300])
        assert await received_bytes(redis, sid) == 300
        await append_chunk(redis, session_id=sid, start=300, chunk=data[300:700])
        assert await received_bytes(redis, sid) == 700


class TestCleanup:
    async def test_cleanup_removes_all_keys(self, redis: FakeRedis) -> None:
        data = b"x" * 100
        sid = await init_session(
            redis,
            task_id=1,
            student_id=2,
            total_size=100,
            declared_sha256=hashlib.sha256(data).hexdigest(),
            filename="x.pdf",
        )
        await append_chunk(redis, session_id=sid, start=0, chunk=data)
        await cleanup_session(redis, sid)

        # 所有相关 keys 已被清空
        keys = await redis.keys(f"upload:chunks:{sid}*")
        assert keys == []
