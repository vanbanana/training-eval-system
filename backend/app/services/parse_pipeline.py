"""Parse Pipeline - 解析引擎主流程.

阶段：
1. fetch upload + storage 内容
2. 调用对应 parser 获得 ParsedDocument（文本提取）
3. 调用 LLM Skill 生成结构化摘要（可选，有 LLM 配置时）
4. SimHash 指纹计算
5. Embedding 向量生成（可选）
6. 持久化到 ParseResult
7. 状态机 pending → parsing → parsed/failed
8. 发布 WebSocket 进度通知
"""

from __future__ import annotations

import hashlib
from typing import TYPE_CHECKING

from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import ResourceNotFoundError
from app.core.logging import get_logger
from app.llm.skills.parse.document_structure import (
    DocumentStructureInput,
    DocumentStructureOutput,
    DocumentStructureSkill,
)
from app.models.upload import ParseResult, Upload
from app.parsers import get_parser
from app.parsers.base import ParsedDocument
from app.repositories.upload_repo import UploadRepository


if TYPE_CHECKING:
    from redis.asyncio import Redis

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
    """解析引擎主管线.

    职责：
    - 从 storage 读取文件内容
    - 路由到对应 parser 提取原始文本
    - 调用 LLM 生成结构化摘要
    - 计算 SimHash + Embedding
    - 持久化 ParseResult
    - 通过 Redis Pub/Sub 推送进度
    """

    def __init__(
        self,
        storage: "FileStorage",
        llm: "LLMProvider | None" = None,
        upload_repo: UploadRepository | None = None,
        redis: "Redis | None" = None,
    ) -> None:
        self.storage = storage
        self.llm = llm
        self.upload_repo = upload_repo or UploadRepository()
        self.redis = redis

    async def _publish_progress(
        self, user_id: int, upload_id: int, status: str, progress: int
    ) -> None:
        """发布解析进度到 WebSocket."""
        if self.redis is None:
            return
        try:
            from app.services.progress_pubsub import publish_progress

            await publish_progress(
                self.redis,
                user_id=user_id,
                upload_id=upload_id,
                status=status,
                progress=progress,
            )
        except Exception as e:  # noqa: BLE001
            log.warning("parse.progress_publish_failed", error=str(e))

    async def run(
        self, db: AsyncSession, *, upload_id: int
    ) -> ParseResult:
        """执行完整解析流程."""
        upload = await self.upload_repo.get(db, upload_id)
        if upload is None:
            raise ResourceNotFoundError(f"upload {upload_id} not found")

        user_id = upload.student_id

        # 状态推进 pending → parsing
        if upload.parse_status == "pending":
            await self.upload_repo.update_status(db, upload_id, "parsing")
            await self._publish_progress(user_id, upload_id, "parsing", 10)

        try:
            # ===== 阶段 1: 文件读取 =====
            log.info("parse.read_file", upload_id=upload_id, path=upload.storage_path)
            content = await self.storage.read_all(upload.storage_path)
            await self._publish_progress(user_id, upload_id, "parsing", 20)

            # ===== 阶段 2: Parser 文本提取 =====
            parser = get_parser(upload.file_type)
            parsed: ParsedDocument = await parser.parse(content)
            log.info(
                "parse.text_extracted",
                upload_id=upload_id,
                text_len=len(parsed.raw_text),
                file_type=upload.file_type,
            )
            await self._publish_progress(user_id, upload_id, "parsing", 40)

            # ===== 阶段 3: LLM 结构化摘要（可选）=====
            structured_content: dict | None = None
            if self.llm is not None and parsed.raw_text:
                structured_content = await self._llm_structure(
                    upload=upload, parsed=parsed
                )
                await self._publish_progress(user_id, upload_id, "parsing", 70)
            else:
                # 无 LLM 时使用 parser 原始输出作为结构化内容
                structured_content = parsed.model_dump()
                await self._publish_progress(user_id, upload_id, "parsing", 70)

            # ===== 阶段 4: SimHash 指纹 =====
            simhash = compute_simhash(parsed.raw_text)

            # ===== 阶段 5: Embedding 向量（可选）=====
            embedding: list[float] | None = None
            if self.llm is not None and parsed.raw_text:
                embedding = await self._compute_embedding(upload_id, parsed.raw_text)
            await self._publish_progress(user_id, upload_id, "parsing", 90)

            # ===== 阶段 6: 持久化 =====
            # 删除旧的 ParseResult（重新解析场景）
            if upload.parse_result is not None:
                await db.delete(upload.parse_result)
                await db.flush()

            result = ParseResult(
                upload_id=upload_id,
                structured_content=structured_content,
                raw_text=parsed.raw_text,
                simhash=simhash,
                embedding=embedding,
            )
            db.add(result)
            await db.flush()

            # 状态推进 parsing → parsed
            await self.upload_repo.update_status(db, upload_id, "parsed")
            await self._publish_progress(user_id, upload_id, "parsed", 100)

            log.info(
                "parse.success",
                upload_id=upload_id,
                text_len=len(parsed.raw_text),
                has_llm_structure=structured_content is not None,
            )
            await db.refresh(result)
            return result

        except Exception as e:
            log.exception("parse.failed", upload_id=upload_id)
            # 推进 → failed
            try:
                await self.upload_repo.update_status(db, upload_id, "failed")
                await self._publish_progress(
                    user_id, upload_id, "failed", 0
                )
            except Exception:  # noqa: BLE001
                pass
            # 记录失败的 ParseResult
            db.add(
                ParseResult(
                    upload_id=upload_id,
                    raw_text="",
                    error_message=str(e),
                )
            )
            await db.flush()
            raise

    async def _llm_structure(
        self, *, upload: Upload, parsed: ParsedDocument
    ) -> dict | None:
        """调用 LLM Skill 生成结构化摘要."""
        if self.llm is None:
            return None

        try:
            # 获取任务要求（用于上下文）
            task_requirements = ""
            # 注意：这里不直接查 DB，由调用方传入或从 upload 关联获取
            # 简化处理：结构化 Skill 不强依赖任务要求

            skill = DocumentStructureSkill()
            input_data = DocumentStructureInput(
                raw_text=parsed.raw_text[:5000],
                file_type=upload.file_type,
                task_requirements=task_requirements,
                filename=upload.filename,
            )
            output: DocumentStructureOutput = await skill.execute(input_data, self.llm)

            # 合并 parser 原始结构 + LLM 摘要
            result = parsed.model_dump()
            result["llm_summary"] = output.summary
            result["llm_sections"] = [s.model_dump() for s in output.sections]
            result["llm_key_topics"] = output.key_topics
            result["llm_completeness"] = output.completeness_assessment
            result["has_code"] = output.has_code
            result["has_diagrams"] = output.has_diagrams
            return result

        except Exception as e:  # noqa: BLE001
            log.warning(
                "parse.llm_structure_failed",
                upload_id=upload.id,
                error=str(e),
            )
            # LLM 失败不阻塞解析，降级为 parser 原始输出
            return parsed.model_dump()

    async def _compute_embedding(
        self, upload_id: int, text: str
    ) -> list[float] | None:
        """计算文本嵌入向量."""
        if self.llm is None:
            return None
        try:
            # 截断到前 5000 字符（embedding 模型通常有 token 限制）
            [embedding] = await self.llm.embed([text[:5000]])
            return embedding
        except Exception as e:  # noqa: BLE001
            log.warning(
                "parse.embed_failed", upload_id=upload_id, error=str(e)
            )
            return None
