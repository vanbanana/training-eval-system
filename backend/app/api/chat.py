"""AI 问答路由（mock 版）."""

from __future__ import annotations

from fastapi import APIRouter
from pydantic import BaseModel, Field
from sqlalchemy import select

from app.api.deps import CurrentUser, DbSession
from app.models.chat import ChatMessage, ChatSession

router = APIRouter(prefix="/api/chat", tags=["chat"])


class SendMessageRequest(BaseModel):
    session_id: int | None = None
    message: str = Field(..., min_length=1, max_length=2000)
    evaluation_id: int | None = None


@router.get("/sessions")
async def list_sessions(db: DbSession, current: CurrentUser) -> list[dict[str, object]]:
    sessions = (await db.execute(
        select(ChatSession).where(ChatSession.student_id == current.id).order_by(ChatSession.created_at.desc()).limit(20)
    )).scalars().all()
    return [{"id": s.id, "title": s.title, "evaluation_id": s.evaluation_id, "created_at": s.created_at.isoformat()} for s in sessions]


@router.get("/sessions/{session_id}/messages")
async def get_messages(session_id: int, db: DbSession, current: CurrentUser) -> list[dict[str, object]]:
    msgs = (await db.execute(
        select(ChatMessage).where(ChatMessage.session_id == session_id).order_by(ChatMessage.created_at)
    )).scalars().all()
    return [{"id": m.id, "role": m.role, "content": m.content, "created_at": m.created_at.isoformat()} for m in msgs]


@router.post("/send")
async def send_message(payload: SendMessageRequest, db: DbSession, current: CurrentUser) -> dict[str, object]:
    """发送消息并获取 AI 回复（非流式，兼容旧调用）— 增强版."""
    from app.services.chat_agent import ChatAgent

    if payload.session_id:
        session = await db.get(ChatSession, payload.session_id)
        if not session:
            from app.core.exceptions import ResourceNotFoundError
            raise ResourceNotFoundError(f"session {payload.session_id} not found")
    else:
        session = ChatSession(student_id=current.id, evaluation_id=payload.evaluation_id, title=payload.message[:30])
        db.add(session)
        await db.flush()

    db.add(ChatMessage(session_id=session.id, role="user", content=payload.message))
    await db.flush()

    agent = ChatAgent()
    full_reply = ""
    async for chunk in agent.stream_reply(db, session=session, user=current):
        full_reply += chunk

    await db.commit()
    return {"session_id": session.id, "reply": full_reply}


@router.post("/stream")
async def stream_message(payload: SendMessageRequest, db: DbSession, current: CurrentUser):
    """流式输出 AI 回复（SSE）— 增强版：多轮上下文 + Function Calling + 角色感知."""
    from fastapi.responses import StreamingResponse

    from app.services.chat_agent import ChatAgent

    if payload.session_id:
        session = await db.get(ChatSession, payload.session_id)
        if not session:
            from app.core.exceptions import ResourceNotFoundError
            raise ResourceNotFoundError(f"session {payload.session_id} not found")
    else:
        session = ChatSession(
            student_id=current.id,
            evaluation_id=payload.evaluation_id,
            title=payload.message[:30],
        )
        db.add(session)
        await db.flush()

    # 保存用户消息
    db.add(ChatMessage(session_id=session.id, role="user", content=payload.message))
    await db.commit()

    agent = ChatAgent()

    async def generate():
        async for event_json in agent.stream_reply(db, session=session, user=current):
            if event_json:
                yield f"data: {event_json}\n\n"

    return StreamingResponse(
        generate(),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Session-Id": str(session.id)},
    )



# ============== Epic 22.5/22.7/22.8 ChatService 标准端点 ==============


class CreateSessionRequest(BaseModel):
    evaluation_id: int | None = None
    title: str = "新对话"


class PostMessageRequest(BaseModel):
    content: str = Field(..., min_length=1, max_length=500)


@router.post("/sessions", status_code=201)
async def create_session_v2(
    payload: CreateSessionRequest, db: DbSession, current: CurrentUser
) -> dict[str, object]:
    from app.services.chat_service import ChatService

    svc = ChatService()
    sess = await svc.create_session(
        db,
        user=current,
        evaluation_id=payload.evaluation_id,
        title=payload.title,
    )
    await db.commit()
    return {
        "id": sess.id,
        "title": sess.title,
        "evaluation_id": sess.evaluation_id,
    }


@router.post("/sessions/{session_id}/messages", status_code=202)
async def post_user_message(
    session_id: int,
    payload: PostMessageRequest,
    db: DbSession,
    current: CurrentUser,
) -> dict[str, object]:
    from app.services.chat_service import ChatService

    svc = ChatService()
    msg = await svc.post_user_message(
        db,
        session_id=session_id,
        content=payload.content,
        user=current,
    )
    await db.commit()
    return {
        "message_id": msg.id,
        "session_id": session_id,
        "ws_topic": f"chat:{session_id}:{msg.id}",
    }


@router.delete("/sessions/{session_id}", status_code=204)
async def delete_session(
    session_id: int, db: DbSession, current: CurrentUser
) -> None:
    from app.services.chat_service import ChatService

    svc = ChatService()
    await svc.soft_delete(db, session_id=session_id, user=current)
    await db.commit()


@router.get("/quota")
async def quota(current: CurrentUser) -> dict[str, object]:
    from app.services.chat_service import ChatService

    svc = ChatService()
    return await svc.quota_status(user_id=current.id)
