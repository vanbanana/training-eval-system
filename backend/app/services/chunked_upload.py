"""Task 8.4: 断点续传支持.

协议：
- 客户端首次：POST /api/uploads/sessions {task_id, total_size, sha256, filename}
  → 返回 upload_session_id
- 客户端续传：PUT /api/uploads/sessions/{id}/chunks
  Header: Content-Range: bytes <start>-<end>/<total>
  Body: chunk bytes
- 客户端完成：POST /api/uploads/sessions/{id}/complete
  → 服务端校验 sha256 + 移交 storage + 创建 Upload 记录

Redis 数据结构：
  upload:chunks:{session_id}                   Hash: filename / total_size / declared_sha256 / task_id / student_id
  upload:chunks:{session_id}:bytes             Sorted set 已收到的 byte ranges
  upload:chunks:{session_id}:data:{start}      Bytes value chunk 实际内容
"""

from __future__ import annotations

import json
import secrets
from dataclasses import dataclass
from typing import TYPE_CHECKING

from app.core.exceptions import BusinessRuleError, ResourceNotFoundError
from app.core.logging import get_logger

if TYPE_CHECKING:
    from redis.asyncio import Redis


log = get_logger(__name__)
_TTL_SECONDS = 24 * 60 * 60


def _meta_key(session_id: str) -> str:
    return f"upload:chunks:{session_id}"


def _data_key(session_id: str, start: int) -> str:
    return f"upload:chunks:{session_id}:data:{start}"


def _ranges_key(session_id: str) -> str:
    return f"upload:chunks:{session_id}:ranges"


@dataclass(slots=True)
class ChunkSession:
    session_id: str
    total_size: int
    declared_sha256: str
    filename: str
    task_id: int
    student_id: int


class ChecksumMismatchError(BusinessRuleError):
    error_code = "CHECKSUM_MISMATCH"


async def init_session(
    redis: Redis,
    *,
    task_id: int,
    student_id: int,
    total_size: int,
    declared_sha256: str,
    filename: str,
) -> str:
    """初始化分片上传会话；返回 session_id."""
    session_id = secrets.token_hex(16)
    meta = {
        "task_id": task_id,
        "student_id": student_id,
        "total_size": total_size,
        "declared_sha256": declared_sha256,
        "filename": filename,
    }
    await redis.set(_meta_key(session_id), json.dumps(meta), ex=_TTL_SECONDS)
    return session_id


async def append_chunk(
    redis: Redis,
    *,
    session_id: str,
    start: int,
    chunk: bytes,
) -> int:
    """追加一个 chunk；返回当前已收到的字节总数."""
    if await redis.get(_meta_key(session_id)) is None:
        raise ResourceNotFoundError(f"upload session {session_id} not found")

    end = start + len(chunk) - 1
    await redis.set(_data_key(session_id, start), chunk, ex=_TTL_SECONDS)
    # 用 zset 记录已收到 ranges（score=start）
    await redis.zadd(_ranges_key(session_id), {f"{start}:{end}": start})
    await redis.expire(_ranges_key(session_id), _TTL_SECONDS)

    # 计算已收到字节总数（去重）
    return await received_bytes(redis, session_id)


async def received_bytes(redis: Redis, session_id: str) -> int:
    ranges = await redis.zrange(_ranges_key(session_id), 0, -1)
    total = 0
    last_end = -1

    def _decode(r: object) -> str:
        return r.decode("latin-1") if isinstance(r, (bytes, bytearray)) else str(r)

    decoded_ranges = sorted(
        (_decode(r) for r in ranges),
        key=lambda s: int(s.split(":")[0]),
    )
    for r in decoded_ranges:
        s_str, e_str = r.split(":")
        s, e = int(s_str), int(e_str)
        if s > last_end:
            total += e - s + 1
            last_end = e
        elif e > last_end:
            total += e - last_end
            last_end = e
    return total


async def assemble(
    redis: Redis,
    *,
    session_id: str,
) -> tuple[bytes, ChunkSession]:
    """合并所有 chunk；校验 sha256；返回完整字节 + 会话元信息."""
    import hashlib

    meta_raw = await redis.get(_meta_key(session_id))
    if meta_raw is None:
        raise ResourceNotFoundError(f"upload session {session_id} not found")
    if isinstance(meta_raw, (bytes, bytearray)):
        meta_raw = meta_raw.decode("utf-8")
    meta = json.loads(meta_raw)
    sess = ChunkSession(
        session_id=session_id,
        total_size=int(meta["total_size"]),
        declared_sha256=str(meta["declared_sha256"]),
        filename=str(meta["filename"]),
        task_id=int(meta["task_id"]),
        student_id=int(meta["student_id"]),
    )

    ranges = await redis.zrange(_ranges_key(session_id), 0, -1)
    if not ranges:
        raise BusinessRuleError("没有任何 chunk", field="session_id")

    def _decode_range(r: object) -> str:
        return r.decode("latin-1") if isinstance(r, (bytes, bytearray)) else str(r)

    sorted_starts = sorted(
        {int(_decode_range(r).split(":")[0]) for r in ranges}
    )
    buf = bytearray()
    cursor = 0
    for s in sorted_starts:
        if s < cursor:
            continue
        data = await redis.get(_data_key(session_id, s))
        if data is None:
            raise BusinessRuleError(f"chunk at {s} 缺失", field="session_id")
        if isinstance(data, str):
            data = data.encode("latin-1")
        buf.extend(data)
        cursor = s + len(data)

    full = bytes(buf)
    if len(full) != sess.total_size:
        raise BusinessRuleError(
            f"组装大小 {len(full)} != 声明 {sess.total_size}",
            field="total_size",
        )

    actual = hashlib.sha256(full).hexdigest()
    if actual != sess.declared_sha256:
        raise ChecksumMismatchError(
            f"sha256 不匹配：expected={sess.declared_sha256} actual={actual}",
            field="sha256",
        )

    return full, sess


async def cleanup_session(redis: Redis, session_id: str) -> None:
    """完成或失败后清理 session 数据."""
    meta_key = _meta_key(session_id)
    ranges_key = _ranges_key(session_id)
    ranges = await redis.zrange(ranges_key, 0, -1)

    def _decode(r: object) -> str:
        return r.decode("latin-1") if isinstance(r, (bytes, bytearray)) else str(r)

    starts = {int(_decode(r).split(":")[0]) for r in ranges}
    keys_to_delete = [meta_key, ranges_key]
    keys_to_delete.extend(_data_key(session_id, s) for s in starts)
    if keys_to_delete:
        await redis.delete(*keys_to_delete)
