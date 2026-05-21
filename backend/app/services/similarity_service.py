"""SimilarityService - Epic 17.

实现：
- compute_simhash / hamming_distance（17.1）
- SimilarityEngine 两阶段比对（17.3）
- find_similar_segments（17.5）
"""

from __future__ import annotations

import difflib
import hashlib
import math
import re

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from app.core.exceptions import ResourceNotFoundError
from app.core.logging import get_logger
from app.models.similarity import SimilarityRecord
from app.models.upload import ParseResult, Upload


log = get_logger(__name__)


# ============== 17.1 SimHash ==============

_TOKEN_RE = re.compile(r"\w+", re.UNICODE)


def _tokenize(text: str) -> list[str]:
    """切词：英文按 \\w+，中文按 2-gram 兜底."""
    tokens: list[str] = []
    for m in _TOKEN_RE.finditer(text or ""):
        tok = m.group()
        if any("\u4e00" <= c <= "\u9fff" for c in tok) and len(tok) >= 2:
            for i in range(len(tok) - 1):
                tokens.append(tok[i : i + 2])
        else:
            tokens.append(tok.lower())
    return tokens


def compute_simhash(text: str, *, hashbits: int = 64) -> int:
    """64-bit SimHash. 返回有符号 64-bit 整数（兼容数据库 BIGINT）."""
    if not text:
        return 0
    tokens = _tokenize(text)
    if not tokens:
        return 0
    v = [0] * hashbits
    for tok in tokens:
        h_bytes = hashlib.md5(tok.encode("utf-8")).digest()
        h = int.from_bytes(h_bytes[:8], byteorder="big")
        for i in range(hashbits):
            bit = (h >> i) & 1
            v[i] += 1 if bit else -1
    fingerprint = 0
    for i in range(hashbits):
        if v[i] > 0:
            fingerprint |= 1 << i
    # 转为有符号 64-bit 兼容 SQLite/PG BIGINT
    if fingerprint >= (1 << 63):
        fingerprint -= 1 << 64
    return fingerprint


def hamming_distance(a: int, b: int) -> int:
    """有符号或无符号皆可：先转为无符号 64-bit 再按位异或."""
    ua = a & ((1 << 64) - 1)
    ub = b & ((1 << 64) - 1)
    return bin(ua ^ ub).count("1")


# ============== 17.5 相似片段 ==============


def find_similar_segments(
    text_a: str, text_b: str, *, min_ratio: float = 0.7, top_k: int = 5
) -> list[dict[str, object]]:
    """基于 SequenceMatcher 找 ratio≥min_ratio 的前 top_k 个连续片段对."""
    if len(text_a) < 50 or len(text_b) < 50:
        return []
    matcher = difflib.SequenceMatcher(a=text_a, b=text_b, autojunk=False)
    blocks = [b for b in matcher.get_matching_blocks() if b.size > 0]
    blocks.sort(key=lambda b: b.size, reverse=True)
    out: list[dict[str, object]] = []
    for b in blocks[:top_k]:
        seg_a = text_a[b.a : b.a + b.size]
        seg_b = text_b[b.b : b.b + b.size]
        ratio = (
            difflib.SequenceMatcher(a=seg_a, b=seg_b).ratio()
            if seg_a and seg_b
            else 0.0
        )
        if ratio >= min_ratio:
            out.append(
                {
                    "a_start": b.a,
                    "a_end": b.a + b.size,
                    "b_start": b.b,
                    "b_end": b.b + b.size,
                    "snippet_a": seg_a,
                    "snippet_b": seg_b,
                    "ratio": round(ratio, 3),
                }
            )
    return out


# ============== 17.3 引擎 ==============


def _cosine(a: list[float], b: list[float]) -> float:
    if not a or not b or len(a) != len(b):
        return 0.0
    dot = sum(x * y for x, y in zip(a, b, strict=True))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    if na == 0 or nb == 0:
        return 0.0
    return dot / (na * nb)


class SimilarityEngine:
    def __init__(
        self,
        *,
        hamming_threshold: int = 6,
        cosine_threshold: float = 0.80,
    ) -> None:
        self.hamming_threshold = hamming_threshold
        self.cosine_threshold = cosine_threshold

    async def detect_for_upload(
        self, db: AsyncSession, *, upload_id: int
    ) -> list[SimilarityRecord]:
        """检测某 upload 与同 task 内其他 upload 的相似度。

        Property 16: 严格按 task_id 限定比对范围。
        """
        upload = (
            await db.execute(
                select(Upload)
                .options(selectinload(Upload.parse_result))
                .where(Upload.id == upload_id)
            )
        ).scalar_one_or_none()
        if upload is None:
            raise ResourceNotFoundError(f"upload {upload_id} not found")
        if upload.parse_result is None or not upload.parse_result.raw_text:
            return []

        # 取本 upload 的指纹
        a_text = upload.parse_result.raw_text
        a_sim = upload.parse_result.simhash
        if a_sim is None:
            a_sim = compute_simhash(a_text)
        a_emb = upload.parse_result.embedding or []

        # 加载同 task 下所有其他已 parsed upload
        rows = list(
            (
                await db.execute(
                    select(Upload)
                    .options(selectinload(Upload.parse_result))
                    .where(
                        Upload.task_id == upload.task_id,
                        Upload.id != upload_id,
                        Upload.parse_status == "parsed",
                    )
                )
            )
            .scalars()
            .all()
        )

        created: list[SimilarityRecord] = []
        for other in rows:
            pr = other.parse_result
            if pr is None or not pr.raw_text:
                continue
            b_sim = pr.simhash if pr.simhash is not None else compute_simhash(pr.raw_text)
            dist = hamming_distance(a_sim, b_sim)
            if dist > self.hamming_threshold:
                continue
            # 阶段 2：余弦
            cosine = None
            if a_emb and pr.embedding:
                cosine = _cosine(a_emb, pr.embedding)
                if cosine < self.cosine_threshold:
                    continue
            # 写入：以 (low, high) 顺序保留唯一约束
            low, high = sorted([upload_id, other.id])
            existing = (
                await db.execute(
                    select(SimilarityRecord).where(
                        SimilarityRecord.task_id == upload.task_id,
                        SimilarityRecord.upload_a_id == low,
                        SimilarityRecord.upload_b_id == high,
                    )
                )
            ).scalar_one_or_none()
            if existing is not None:
                continue
            rec = SimilarityRecord(
                task_id=upload.task_id,
                upload_a_id=low,
                upload_b_id=high,
                hamming_distance=dist,
                cosine_similarity=cosine,
                state="suspect",
            )
            db.add(rec)
            created.append(rec)
        await db.flush()
        log.info(
            "similarity.detected",
            upload_id=upload_id,
            count=len(created),
        )
        return created


__all__ = [
    "SimilarityEngine",
    "compute_simhash",
    "find_similar_segments",
    "hamming_distance",
]
