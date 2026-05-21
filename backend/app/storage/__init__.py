"""文件存储抽象."""

from app.storage.base import (
    FileSizeExceededError,
    FileStorageNotFoundError,
    FileWriteError,
    StorageError,
)
from app.storage.local_fs import LocalFileStorage

__all__ = [
    "FileSizeExceededError",
    "FileStorageNotFoundError",
    "FileWriteError",
    "LocalFileStorage",
    "StorageError",
]
