"""Task 7.1 / 7.2 验收：FileStorage Protocol + LocalFileStorage."""

from __future__ import annotations

import hashlib
from pathlib import Path

import pytest

from app.storage.base import (
    FileStorage,
    FileStorageNotFoundError,
    FileWriteError,
)
from app.storage.local_fs import LocalFileStorage


pytestmark = pytest.mark.unit


@pytest.fixture()
def storage(tmp_path: Path) -> LocalFileStorage:
    return LocalFileStorage(tmp_path)


class TestProtocolConformance:
    def test_local_storage_implements_protocol(
        self, storage: LocalFileStorage
    ) -> None:
        assert isinstance(storage, FileStorage)


class TestSaveSha256:
    async def test_save_returns_correct_sha256(
        self, storage: LocalFileStorage
    ) -> None:
        data = b"x" * (1024 * 1024)  # 1MB
        digest = await storage.save("a/b/c.bin", data)
        expected = hashlib.sha256(data).hexdigest()
        assert digest == expected

    async def test_saved_file_can_be_read(self, storage: LocalFileStorage) -> None:
        data = b"hello world"
        await storage.save("test.txt", data)
        assert await storage.read_all("test.txt") == data


class TestPathTraversal:
    async def test_dotdot_rejected(self, storage: LocalFileStorage) -> None:
        with pytest.raises(ValueError, match="\\.\\."):
            await storage.save("../escape.txt", b"x")

    async def test_absolute_path_rejected(self, storage: LocalFileStorage) -> None:
        with pytest.raises(ValueError, match="相对路径"):
            await storage.save("/etc/passwd", b"x")

    async def test_exists_does_not_raise_on_invalid_path(
        self, storage: LocalFileStorage
    ) -> None:
        assert await storage.exists("../bad") is False


class TestExists:
    async def test_returns_true_after_save(
        self, storage: LocalFileStorage
    ) -> None:
        await storage.save("a.txt", b"hello")
        assert await storage.exists("a.txt") is True

    async def test_returns_false_for_nonexistent(
        self, storage: LocalFileStorage
    ) -> None:
        assert await storage.exists("nope.txt") is False


class TestSize:
    async def test_returns_byte_count(self, storage: LocalFileStorage) -> None:
        await storage.save("size.bin", b"abc" * 10)
        assert await storage.size("size.bin") == 30

    async def test_raises_on_missing(self, storage: LocalFileStorage) -> None:
        with pytest.raises(FileStorageNotFoundError):
            await storage.size("missing")


class TestDelete:
    async def test_returns_true_on_existing(
        self, storage: LocalFileStorage
    ) -> None:
        await storage.save("d.bin", b"x")
        assert await storage.delete("d.bin") is True
        assert await storage.exists("d.bin") is False

    async def test_returns_false_on_missing(
        self, storage: LocalFileStorage
    ) -> None:
        assert await storage.delete("nope") is False


class TestAtomicWrite:
    async def test_no_temp_file_left_after_success(
        self, storage: LocalFileStorage, tmp_path: Path
    ) -> None:
        await storage.save("a/b.bin", b"data")
        # 确认没有 .tmp_ 临时文件残留
        leftover = list(tmp_path.rglob(".tmp_*"))
        assert leftover == []


class TestStreamRead:
    async def test_open_yields_full_content(
        self, storage: LocalFileStorage
    ) -> None:
        data = b"chunk-test " * 100
        await storage.save("stream.bin", data)
        gen = await storage.open("stream.bin")
        collected = b""
        async for chunk in gen:
            collected += chunk
        assert collected == data

    async def test_open_raises_for_missing(
        self, storage: LocalFileStorage
    ) -> None:
        with pytest.raises(FileStorageNotFoundError):
            await storage.open("nope")


class TestSha256:
    async def test_sha256_matches_save_return(
        self, storage: LocalFileStorage
    ) -> None:
        data = b"test-hash-" * 100
        digest_save = await storage.save("h.bin", data)
        digest_query = await storage.sha256("h.bin")
        assert digest_save == digest_query
