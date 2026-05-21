"""LocalFileStorage - 本地文件系统实现.

特性：
- aiofiles 异步 IO
- save 用临时文件 + os.replace 原子重命名（避免半截文件）
- sha256 边写边算
- path traversal 防护：拒绝包含 `..` 或绝对路径的输入
"""

from __future__ import annotations

import hashlib
import os
import uuid
from collections.abc import AsyncIterator
from pathlib import Path

import aiofiles
import aiofiles.os

from app.core.logging import get_logger
from app.storage.base import (
    FileStorageNotFoundError,
    FileWriteError,
)

log = get_logger(__name__)

# 流式读取块大小
_CHUNK_SIZE = 64 * 1024


class LocalFileStorage:
    """根目录下的本地文件存储."""

    def __init__(self, root: str | Path) -> None:
        self.root = Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True)

    # ============ 内部 ============

    def _resolve(self, rel_path: str) -> Path:
        """安全解析相对路径；阻止 path traversal."""
        if not rel_path or rel_path.startswith(("/", "\\")):
            raise ValueError("path 必须是相对路径")
        if ".." in Path(rel_path).parts:
            raise ValueError("path 不允许包含 ..")
        target = (self.root / rel_path).resolve()
        try:
            target.relative_to(self.root)
        except ValueError as e:
            raise ValueError("path 越出存储根目录") from e
        return target

    # ============ 接口实现 ============

    async def save(self, path: str, data: bytes) -> str:
        """原子写入：先写临时文件、计算 sha256、再 rename 到目标."""
        target = self._resolve(path)
        target.parent.mkdir(parents=True, exist_ok=True)

        tmp_name = f".tmp_{uuid.uuid4().hex}"
        tmp_path = target.parent / tmp_name
        sha = hashlib.sha256()
        try:
            async with aiofiles.open(tmp_path, "wb") as f:
                # 分块写入便于将来支持流（当前 bytes 全部一次写）
                view = memoryview(data)
                offset = 0
                while offset < len(view):
                    chunk = bytes(view[offset : offset + _CHUNK_SIZE])
                    await f.write(chunk)
                    sha.update(chunk)
                    offset += _CHUNK_SIZE
            # 原子重命名
            os.replace(tmp_path, target)
            return sha.hexdigest()
        except Exception as e:
            # 清理临时文件
            try:
                if tmp_path.exists():
                    tmp_path.unlink()
            except OSError:
                pass
            raise FileWriteError(f"写入失败: {e}", field="path") from e

    async def open(self, path: str) -> AsyncIterator[bytes]:
        target = self._resolve(path)
        if not target.exists():
            raise FileStorageNotFoundError(f"file {path} not found")

        async def _gen() -> AsyncIterator[bytes]:
            async with aiofiles.open(target, "rb") as f:
                while True:
                    chunk = await f.read(_CHUNK_SIZE)
                    if not chunk:
                        break
                    yield chunk

        return _gen()

    async def read_all(self, path: str) -> bytes:
        target = self._resolve(path)
        if not target.exists():
            raise FileStorageNotFoundError(f"file {path} not found")
        async with aiofiles.open(target, "rb") as f:
            return await f.read()

    async def exists(self, path: str) -> bool:
        try:
            target = self._resolve(path)
        except ValueError:
            return False
        return target.exists() and target.is_file()

    async def delete(self, path: str) -> bool:
        target = self._resolve(path)
        if not target.exists():
            return False
        try:
            await aiofiles.os.remove(target)
            return True
        except OSError as e:
            raise FileWriteError(f"删除失败: {e}", field="path") from e

    async def size(self, path: str) -> int:
        target = self._resolve(path)
        if not target.exists():
            raise FileStorageNotFoundError(f"file {path} not found")
        return target.stat().st_size

    async def sha256(self, path: str) -> str:
        target = self._resolve(path)
        if not target.exists():
            raise FileStorageNotFoundError(f"file {path} not found")
        sha = hashlib.sha256()
        async with aiofiles.open(target, "rb") as f:
            while True:
                chunk = await f.read(_CHUNK_SIZE)
                if not chunk:
                    break
                sha.update(chunk)
        return sha.hexdigest()
