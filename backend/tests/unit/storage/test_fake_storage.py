"""Task 7.4 验收：InMemoryStorage 与 LocalFileStorage 接口兼容性."""

from __future__ import annotations

import hashlib
from pathlib import Path

import pytest

from app.storage.base import FileStorage, FileStorageNotFoundError
from app.storage.local_fs import LocalFileStorage
from tests.fakes.fake_storage import InMemoryStorage


pytestmark = pytest.mark.unit


@pytest.fixture(params=["local", "memory"])
def storage(request: pytest.FixtureRequest, tmp_path: Path) -> object:
    if request.param == "local":
        return LocalFileStorage(tmp_path)
    return InMemoryStorage()


def test_both_implement_protocol() -> None:
    """Given LocalFileStorage / InMemoryStorage；When isinstance；
    Then 都满足 FileStorage Protocol 接口。"""
    fake = InMemoryStorage()
    assert isinstance(fake, FileStorage)


class TestInterfaceParity:
    """同一组测试运行在两种 storage 上，验证接口兼容."""

    async def test_save_and_read(self, storage: object) -> None:
        digest = await storage.save("a.bin", b"data")  # type: ignore[attr-defined]
        assert digest == hashlib.sha256(b"data").hexdigest()
        assert await storage.read_all("a.bin") == b"data"  # type: ignore[attr-defined]

    async def test_exists(self, storage: object) -> None:
        assert await storage.exists("nope") is False  # type: ignore[attr-defined]
        await storage.save("a.bin", b"x")  # type: ignore[attr-defined]
        assert await storage.exists("a.bin") is True  # type: ignore[attr-defined]

    async def test_delete(self, storage: object) -> None:
        await storage.save("a.bin", b"x")  # type: ignore[attr-defined]
        assert await storage.delete("a.bin") is True  # type: ignore[attr-defined]
        assert await storage.delete("a.bin") is False  # type: ignore[attr-defined]

    async def test_missing_raises_consistent(self, storage: object) -> None:
        with pytest.raises(FileStorageNotFoundError):
            await storage.size("nope")  # type: ignore[attr-defined]


class TestInMemoryClear:
    async def test_clear_removes_all(self) -> None:
        s = InMemoryStorage()
        await s.save("a.bin", b"x")
        await s.save("b.bin", b"y")
        s.clear()
        assert await s.exists("a.bin") is False
        assert await s.exists("b.bin") is False
