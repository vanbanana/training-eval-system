"""FileStorage Protocol 与异常."""

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Protocol, runtime_checkable

from app.core.exceptions import BusinessRuleError, ExternalServiceError


class StorageError(ExternalServiceError):
    error_code = "STORAGE_ERROR"


class FileStorageNotFoundError(StorageError):
    error_code = "FILE_NOT_FOUND"
    http_status = 404


class FileSizeExceededError(BusinessRuleError):
    error_code = "FILE_SIZE_EXCEEDED"


class FileWriteError(StorageError):
    error_code = "FILE_WRITE_FAILED"


@runtime_checkable
class FileStorage(Protocol):
    """文件存储接口.

    路径策略：调用方自行构造 `task_{task_id}/student_{student_id}/{uuid}.{ext}`，
    存储层不负责业务路径生成。
    """

    async def save(self, path: str, data: bytes) -> str:
        """保存数据；返回 sha256 hex digest."""
        ...

    async def open(self, path: str) -> AsyncIterator[bytes]:
        """流式读文件；不存在抛 FileStorageNotFoundError."""
        ...

    async def read_all(self, path: str) -> bytes:
        ...

    async def exists(self, path: str) -> bool:
        ...

    async def delete(self, path: str) -> bool:
        ...

    async def size(self, path: str) -> int:
        ...

    async def sha256(self, path: str) -> str:
        ...
