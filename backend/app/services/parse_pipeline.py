"""Parse Pipeline - 解析引擎主流程.

阶段：
1. fetch upload + storage 内容
2. 调用对应 parser 获得 ParsedDocument
3. SimHash + Embedding（可选 LLM 调用）
4. 持久化到 ParseResult
5. 状态机 pending → parsing → parsed/failed
"""

from __future__ import annotations

import hashlib
from typing import TYPE_CHECKING

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import ResourceNotFoundError
from app.core.logging import get_logger
from app.models.upload import ParseResult, Upload
from app.parsers import get_parser
from app.parsers.base import ParsedDocument
from app.repositories.upload_repo import UploadRepository


if TYPE_CHECKING:
    from app.llm.base import LLMProvider
    from app.storage.base import FileStorage


log = get_logger(__name__)


def compute_simhash(text: str, *, hash_bits: int = 64) -> int:
    """简化的 SimHash 实现：用 hash 函数生成 64 位指纹（带符号 64 位整数）."""
    if not text:
        return 0
    # 滑动窗口分片
    tokens = [text[i : i + 4] for i in range(0, max(1, len(text) - 3))]
    if not tokens:
        return 0
    bits = [0] * hash_bits
    for token in tokens:
        h = int.from_bytes(
            hashlib.sha256(token.encode("utf-8")).digest()[:8], "big"
        )
        for i in range(hash_bits):
            if h & (1 << i):
                bits[i] += 1
            else:
                bits[i] -= 1
    fingerprint = 0
    for i, b in enumerate(bits):
        if b > 0:
            fingerprint |= 1 << i
    # 转为带符号 64 位整数（DB BigInteger 限制）
    if fingerprint >= (1 << 63):
        fingerprint -= 1 << 64
    return fingerprint


class ParsePipeline:
    def __init__(
        self,
        storage: "FileStorage",
        llm: "LLMProvider | None" = None,
        upload_repo: UploadRepository | None = None,
    ) -> None:
        self.storage = storage
        self.llm = llm
        self.upload_repo = upload_repo or UploadRepository()

    async def run(
        self, db: AsyncSession, *, upload_id: int
    ) -> ParseResult:
        upload = await self.upload_repo.get(db, upload_id)
        if upload is None:
            raise ResourceNotFoundError(f"upload {upload_id} not found")

        # 状态推进 pending → parsing
        if upload.parse_status == "pending":
            await self.upload_repo.update_status(db, upload_id, "parsing")

        try:
            content = await self.storage.read_all(upload.storage_path)
            parser = get_parser(upload.file_type)
            parsed: ParsedDocument = await parser.parse(content)

            simhash = compute_simhash(parsed.raw_text)

            embedding: list[float] | None = None
            if self.llm is not None and parsed.raw_text:
                try:
                    [embedding] = await self.llm.embed([parsed.raw_text[:5000]])
                except Exception as e:  # noqa: BLE001
                    log.warning(
                        "parse.embed_failed", upload_id=upload_id, error=str(e)
                    )

            result = ParseResult(
                upload_id=upload_id,
                structured_content=parsed.model_dump(),
                raw_text=parsed.raw_text,
                simhash=simhash,
                embedding=embedding,
            )
            db.add(result)
            await db.flush()

            # 状态推进 parsing → parsed
            await self.upload_repo.update_status(db, upload_id, "parsed")
            log.info(
                "parse.success", upload_id=upload_id, text_len=len(parsed.raw_text)
            )
            await db.refresh(result)
            return result
        except Exception as e:
            log.exception("parse.failed", upload_id=upload_id)
            # 推进 → failed
            try:
                await self.upload_repo.update_status(db, upload_id, "failed")
            except Exception:  # noqa: BLE001
                pass
            db.add(
                ParseResult(
                    upload_id=upload_id,
                    raw_text="",
                    error_message=str(e),
                )
            )
            await db.flush()
            raise
