"""ChatService - Epic 22.4."""

from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.exceptions import (
    AuthorizationError,
    BusinessRuleError,
    RateLimitedError,
    ResourceNotFoundError,
)
from app.core.logging import get_logger
from app.llm.base import LLMMessage, LLMProvider
from app.models.chat import ChatMessage, ChatSession
from app.models.user import User


log = get_logger(__name__)


SESSION_LIMIT_ERROR_CODE = "SESSION_LIMIT_EXCEEDED"
DEFAULT_DAILY_QUOTA = 50
MAX_USER_MESSAGE_LEN = 500
MAX_USER_MESSAGES_PER_SESSION = 20


class SessionLimitExceededError(BusinessRuleError):
    error_code = SESSION_LIMIT_ERROR_CODE


class ChatService:
    def __init__(
        self,
        llm: LLMProvider | None = None,
        *,
        daily_quota: int = DEFAULT_DAILY_QUOTA,
    ) -> None:
        self.llm = llm
        self.daily_quota = daily_quota
        self._quota_used: dict[int, int] = {}  # 测试可重置；生产用 Redis

    async def create_session(
        self,
        db: AsyncSession,
        *,
        user: User,
        evaluation_id: int | None = None,
        title: str = "新对话",
    ) -> ChatSession:
        if evaluation_id is not None:
            from app.models.evaluation import Evaluation

            ev = await db.get(Evaluation, evaluation_id)
            if ev is None:
                raise ResourceNotFoundError(
                    f"evaluation {evaluation_id} not found"
                )
            if ev.student_id != user.id and user.role != "admin":
                raise AuthorizationError("无权基于他人评价开会话")
        session = ChatSession(
            student_id=user.id,
            evaluation_id=evaluation_id,
            title=title,
        )
        db.add(session)
        await db.flush()
        return session

    async def list_sessions(
        self, db: AsyncSession, *, user: User
    ) -> list[ChatSession]:
        return list(
            (
                await db.execute(
                    select(ChatSession)
                    .where(
                        ChatSession.student_id == user.id,
                        ChatSession.is_deleted.is_(False),
                    )
                    .order_by(ChatSession.last_active_at.desc())
                )
            )
            .scalars()
            .all()
        )

    async def list_messages(
        self,
        db: AsyncSession,
        *,
        session_id: int,
        user: User,
        limit: int = 50,
    ) -> list[ChatMessage]:
        sess = await db.get(ChatSession, session_id)
        if sess is None or sess.is_deleted:
            raise ResourceNotFoundError(f"session {session_id} not found")
        if sess.student_id != user.id and user.role != "admin":
            raise AuthorizationError("无权访问他人会话")
        return list(
            (
                await db.execute(
                    select(ChatMessage)
                    .where(ChatMessage.session_id == session_id)
                    .order_by(ChatMessage.created_at)
                    .limit(limit)
                )
            )
            .scalars()
            .all()
        )

    async def soft_delete(
        self, db: AsyncSession, *, session_id: int, user: User
    ) -> None:
        sess = await db.get(ChatSession, session_id)
        if sess is None:
            raise ResourceNotFoundError(f"session {session_id} not found")
        if sess.student_id != user.id and user.role != "admin":
            raise AuthorizationError("无权删除他人会话")
        sess.is_deleted = True
        await db.flush()

    async def quota_status(self, *, user_id: int) -> dict[str, Any]:
        used = self._quota_used.get(user_id, 0)
        return {
            "used": used,
            "limit": self.daily_quota,
            "remaining": max(0, self.daily_quota - used),
        }

    async def post_user_message(
        self,
        db: AsyncSession,
        *,
        session_id: int,
        content: str,
        user: User,
    ) -> ChatMessage:
        # 内容长度
        if len(content) > MAX_USER_MESSAGE_LEN:
            raise BusinessRuleError(
                f"消息长度不可超过 {MAX_USER_MESSAGE_LEN}", field="content"
            )
        # 配额
        used = self._quota_used.get(user.id, 0)
        if used >= self.daily_quota:
            raise RateLimitedError("今日 AI 问答配额已用尽")

        sess = await db.get(ChatSession, session_id)
        if sess is None or sess.is_deleted:
            raise ResourceNotFoundError(f"session {session_id} not found")
        if sess.student_id != user.id:
            raise AuthorizationError("无权访问他人会话")

        # 单 session 最多 20 条 user 消息
        existing = list(
            (
                await db.execute(
                    select(ChatMessage).where(
                        ChatMessage.session_id == session_id,
                        ChatMessage.role == "user",
                    )
                )
            )
            .scalars()
            .all()
        )
        if len(existing) >= MAX_USER_MESSAGES_PER_SESSION:
            raise SessionLimitExceededError(
                f"会话最多 {MAX_USER_MESSAGES_PER_SESSION} 条用户消息"
            )

        msg = ChatMessage(
            session_id=session_id,
            role="user",
            content=content,
        )
        db.add(msg)
        self._quota_used[user.id] = used + 1
        await db.flush()
        return msg

    async def answer_stream(
        self,
        db: AsyncSession,
        *,
        session_id: int,
        user: User,
    ) -> AsyncIterator[str]:
        """读取最近用户消息，流式生成 assistant 回答."""
        msgs = await self.list_messages(
            db, session_id=session_id, user=user, limit=40
        )
        if not msgs or msgs[-1].role != "user":
            yield "[no-pending-message]"
            return
        if self.llm is None:
            content = "（dev fake）已收到你的问题"
            db.add(
                ChatMessage(
                    session_id=session_id,
                    role="assistant",
                    content=content,
                )
            )
            await db.flush()
            yield content
            return

        provider_msgs = [
            LLMMessage(role=m.role, content=m.content)  # type: ignore[arg-type]
            for m in msgs
            if m.role in ("user", "assistant", "system")
        ]
        buffer = ""
        try:
            async for delta in self.llm.chat_stream(provider_msgs):
                buffer += delta
                yield delta
        except Exception as e:  # noqa: BLE001
            log.warning("chat.stream_interrupted", error=str(e))
            buffer += " [stream_interrupted]"
        # 持久化 assistant
        db.add(
            ChatMessage(
                session_id=session_id,
                role="assistant",
                content=buffer,
            )
        )
        await db.flush()
