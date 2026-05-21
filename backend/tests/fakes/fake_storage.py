"""InMemoryStorage - 测试替身（接口与 LocalFileStorage 对齐）."""

from __future__ import annotations

import hashlib
from collections.abc import AsyncIterator

from app.storage.base import FileStorageNotFoundError


class InMemoryStorage:
    def __init__(self) -> None:
        self._store: dict[str, bytes] = {}

    def clear(self) -> None:
        self._store.clear()

    async def save(self, path: str, data: bytes) -> str:
        if ".." in path or path.startswith(("/", "\\")):
            raise ValueError("path 非法")
        self._store[path] = bytes(data)
        return hashlib.sha256(data).hexdigest()

    async def open(self, path: str) -> AsyncIterator[bytes]:
        data = self._store.get(path)
        if data is None:
            raise FileStorageNotFoundError(f"file {path} not found")

        async def _gen() -> AsyncIterator[bytes]:
            yield data

        return _gen()

    async def read_all(self, path: str) -> bytes:
        if path not in self._store:
            raise FileStorageNotFoundError(f"file {path} not found")
        return self._store[path]

    async def exists(self, path: str) -> bool:
        return path in self._store

    async def delete(self, path: str) -> bool:
        return self._store.pop(path, None) is not None

    async def size(self, path: str) -> int:
        if path not in self._store:
            raise FileStorageNotFoundError(f"file {path} not found")
        return len(self._store[path])

    async def sha256(self, path: str) -> str:
        if path not in self._store:
            raise FileStorageNotFoundError(f"file {path} not found")
        return hashlib.sha256(self._store[path]).hexdigest()
