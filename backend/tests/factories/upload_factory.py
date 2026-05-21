"""UploadFactory."""

from __future__ import annotations

import hashlib
import uuid
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.models.task import TrainingTask
from app.models.upload import Upload
from app.models.user import User
from tests.factories import faker
from tests.factories.task_factory import TrainingTaskFactory
from tests.factories.user_factory import UserFactory


# 内置 sample 字节
SAMPLES: dict[str, bytes] = {
    "sample.pdf": b"%PDF-1.4\n" + b"x" * 2000,
    "sample.docx": b"PK\x03\x04" + b"x" * 2000,
    "sample.png": b"\x89PNG\r\n\x1a\n" + b"x" * 2000,
    "tampered.pdf": b"\x89PNG\r\n\x1a\n" + b"x" * 2000,  # 实际是 png 改名为 pdf
    "large.bin": b"PK\x03\x04" + b"x" * (10 * 1024),
}


class UploadFactory:
    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        student: User | None = None,
        task: TrainingTask | None = None,
        filename: str | None = None,
        file_type: str = "docx",
        file_size: int = 10000,
        parse_status: str = "parsed",
        sha256: str | None = None,
        version: int = 1,
        **extra: Any,
    ) -> Upload:
        if student is None:
            student = await UserFactory.create_async(
                session, username=f"u_{faker.uuid4()[:6]}"
            )
        if task is None:
            task = await TrainingTaskFactory.create_async(session)

        sha = sha256 or hashlib.sha256(uuid.uuid4().bytes).hexdigest()
        upload = Upload(
            task_id=task.id,
            student_id=student.id,
            filename=filename or f"file.{file_type}",
            file_type=file_type,
            file_size=file_size,
            storage_path=f"task_{task.id}/student_{student.id}/{uuid.uuid4().hex}.{file_type}",
            sha256=sha,
            parse_status=parse_status,
            version=version,
            **extra,
        )
        session.add(upload)
        await session.flush()
        await session.refresh(upload)
        return upload


def get_sample(name: str) -> bytes:
    """取得内置示例文件字节内容."""
    if name not in SAMPLES:
        raise KeyError(f"unknown sample {name}; available: {list(SAMPLES)}")
    return SAMPLES[name]
