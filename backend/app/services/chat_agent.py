"""ChatAgent - 增强版 AI 对话引擎.

参考 nanoclaw-py 的 agent 架构，实现：
1. 多轮上下文（完整对话历史发送给 LLM）
2. 角色感知系统提示（学生/教师/管理员不同 prompt）
3. Function Calling（LLM 可调用系统工具获取真实数据）
4. 用户上下文注入（评价数据、薄弱点、班级信息等）
5. 结构化 SSE 事件（思考/工具调用/工具结果/文本/完成）
6. 对话摘要压缩（长对话自动压缩前文）

不依赖 Claude Code SDK，使用 OpenAI 兼容 API + Function Calling。

SSE 事件协议：
  {"type":"thinking","content":"..."}     — AI 正在思考/分析
  {"type":"tool_call","name":"...","args":{...}}  — 调用工具
  {"type":"tool_result","name":"...","success":bool,"data":{...}} — 工具返回
  {"type":"text","content":"..."}         — 文本流式输出（增量）
  {"type":"done","full_content":"..."}    — 完成，附完整文本
  {"type":"error","message":"..."}        — 错误
"""

from __future__ import annotations

import json
from collections.abc import AsyncIterator
from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.logging import get_logger
from app.llm.tools.base import Tool, ToolExecutionContext, ToolResult
from app.llm.tools.registry import tool_registry
from app.models.chat import ChatMessage, ChatSession
from app.models.user import User

log = get_logger(__name__)

MAX_HISTORY_MESSAGES = 20
MAX_TOOL_CALLS_PER_TURN = 5
SUMMARY_THRESHOLD = 12  # 超过此条数时压缩前文


# ============ System Prompts (角色感知) ============

_STUDENT_SYSTEM_PROMPT = """你是「智能实训评价管理系统」的 AI 学习助手。你正在与一位学生对话。

## 你的能力
- 帮助学生理解评价结果、分析薄弱点、给出改进建议
- 可以调用系统工具查询学生的真实数据（评分详情、维度历史、班级统计等）
- 基于学生的历史评价数据给出个性化建议

## 当前学生信息
{user_context}

## 对话规则
- 回答简洁专业，使用中文
- 如果需要查询数据来回答问题，主动调用工具
- 不要编造数据，如果工具返回错误就如实告知
- 鼓励学生，给出具体可操作的改进建议
- 如果学生问的问题超出系统范围，礼貌说明并引导回实训相关话题
"""

_TEACHER_SYSTEM_PROMPT = """你是「智能实训评价管理系统」的 AI 教学助手。你正在与一位教师对话。

## 你的能力
- 帮助教师分析班级整体表现、识别教学薄弱环节
- 提供评分标准建议、维度权重调整建议
- 协助教师理解 AI 评分逻辑和结果

## 当前教师信息
{user_context}

## 对话规则
- 回答专业严谨，使用中文
- 提供数据驱动的教学建议
- 尊重教师的专业判断，AI 建议仅供参考
- 涉及具体学生时注意隐私保护
"""

_ADMIN_SYSTEM_PROMPT = """你是「智能实训评价管理系统」的 AI 运维助手。你正在与系统管理员对话。

## 你的能力
- 帮助管理员了解系统运行状态
- 解答系统配置、用户管理、LLM 配置相关问题
- 提供系统优化建议

## 当前系统信息
{user_context}

## 对话规则
- 回答技术性强，使用中文
- 涉及安全敏感操作时提醒确认
- 不直接执行危险操作（删库、重置密码等），仅提供指导
"""


# ============ Context Builder ============

async def _build_student_context(db: AsyncSession, user: User) -> str:
    """构建学生上下文信息（注入到 system prompt）."""
    from app.models.evaluation import Evaluation
    from sqlalchemy import func as sqlfunc

    lines: list[str] = []
    lines.append(f"- 姓名：{user.display_name}")
    lines.append(f"- 用户名：{user.username}")

    # 评价统计
    eval_count = (
        await db.execute(
            select(sqlfunc.count(Evaluation.id)).where(
                Evaluation.student_id == user.id,
                Evaluation.total_score.isnot(None),
            )
        )
    ).scalar() or 0
    lines.append(f"- 已完成评价次数：{eval_count}")

    if eval_count > 0:
        avg_score = (
            await db.execute(
                select(sqlfunc.avg(Evaluation.total_score)).where(
                    Evaluation.student_id == user.id,
                    Evaluation.total_score.isnot(None),
                )
            )
        ).scalar()
        if avg_score:
            lines.append(f"- 历史平均分：{round(float(avg_score), 1)}")

        # 最近一次评价
        latest = (
            await db.execute(
                select(Evaluation)
                .where(
                    Evaluation.student_id == user.id,
                    Evaluation.total_score.isnot(None),
                )
                .order_by(Evaluation.created_at.desc())
                .limit(1)
            )
        ).scalar_one_or_none()
        if latest:
            lines.append(f"- 最近评分：{latest.total_score}（任务 ID: {latest.task_id}）")

    # 薄弱点
    try:
        from app.models.evaluation import DimensionScore
        from app.models.task import Dimension

        dim_scores = list(
            (
                await db.execute(
                    select(Dimension.name, sqlfunc.avg(DimensionScore.ai_score))
                    .join(DimensionScore, DimensionScore.dimension_id == Dimension.id)
                    .join(Evaluation, Evaluation.id == DimensionScore.evaluation_id)
                    .where(
                        Evaluation.student_id == user.id,
                        DimensionScore.ai_score.isnot(None),
                    )
                    .group_by(Dimension.name)
                    .order_by(sqlfunc.avg(DimensionScore.ai_score).asc())
                    .limit(3)
                )
            ).all()
        )
        if dim_scores:
            weaknesses = [f"{name}({round(float(score), 1)}分)" for name, score in dim_scores]
            lines.append(f"- 薄弱维度：{', '.join(weaknesses)}")
    except Exception:  # noqa: BLE001
        pass

    return "\n".join(lines) if lines else "暂无数据"


async def _build_teacher_context(db: AsyncSession, user: User) -> str:
    """构建教师上下文信息."""
    from app.models.course import Class
    from app.models.task import TrainingTask
    from sqlalchemy import func as sqlfunc

    lines: list[str] = []
    lines.append(f"- 姓名：{user.display_name}")

    task_count = (
        await db.execute(
            select(sqlfunc.count(TrainingTask.id)).where(
                TrainingTask.teacher_id == user.id
            )
        )
    ).scalar() or 0
    lines.append(f"- 管理任务数：{task_count}")

    class_count = (
        await db.execute(
            select(sqlfunc.count(Class.id)).where(Class.teacher_id == user.id)
        )
    ).scalar() or 0
    lines.append(f"- 管理班级数：{class_count}")

    return "\n".join(lines)


async def _build_admin_context(db: AsyncSession, user: User) -> str:
    """构建管理员上下文信息."""
    from app.models.task import TrainingTask
    from sqlalchemy import func as sqlfunc

    lines: list[str] = []
    lines.append(f"- 管理员：{user.display_name}")

    user_count = (
        await db.execute(select(sqlfunc.count(User.id)))
    ).scalar() or 0
    lines.append(f"- 系统用户数：{user_count}")

    task_count = (
        await db.execute(select(sqlfunc.count(TrainingTask.id)))
    ).scalar() or 0
    lines.append(f"- 系统任务数：{task_count}")

    return "\n".join(lines)


# ============ ChatAgent 核心 ============

class ChatAgent:
    """增强版 AI 对话引擎.

    核心流程：
    1. 加载会话历史（多轮上下文）
    2. 构建角色感知 system prompt + 用户上下文
    3. 注册可用工具（Function Calling）
    4. 调用 LLM，处理 tool_calls 循环
    5. 流式返回最终回复
    """

    def __init__(self, *, max_tool_rounds: int = MAX_TOOL_CALLS_PER_TURN) -> None:
        self.max_tool_rounds = max_tool_rounds

    async def stream_reply(
        self,
        db: AsyncSession,
        *,
        session: ChatSession,
        user: User,
    ) -> AsyncIterator[str]:
        """流式生成 AI 回复（含 Function Calling 循环）.

        Yields 结构化 JSON 事件字符串（前端解析后渲染不同 UI 组件）。
        """
        from openai import AsyncOpenAI

        from app.llm.client import get_active_config

        config = await get_active_config(db)
        if not config:
            yield self._event("error", message="未配置 LLM 服务，请联系管理员在系统设置中配置 API Key")
            yield self._event("done", full_content="")
            return

        # 1. 构建 system prompt
        yield self._event("thinking", content="正在加载对话上下文...")
        system_prompt = await self._build_system_prompt(db, user, session)

        # 2. 加载对话历史（含摘要压缩）
        import asyncio
        await asyncio.sleep(0.6)
        yield self._event("thinking", content="正在分析用户画像与历史数据...")
        await asyncio.sleep(0.5)
        history = await self._load_history_with_summary(db, session.id)

        # 3. 获取可用工具
        tools_schema = self._get_tools_for_role(user.role)
        tool_ctx = ToolExecutionContext(
            actor_id=user.id,
            actor_role=user.role,
            evaluation_id=session.evaluation_id,
            student_id=user.id if user.role == "student" else None,
            db=db,
        )

        # 4. 构建 messages
        messages: list[dict[str, Any]] = [
            {"role": "system", "content": system_prompt},
        ]
        messages.extend(history)

        # 5. LLM 调用（含 tool_calls 循环）
        client = AsyncOpenAI(
            api_key=config.api_key_encrypted,
            base_url=config.base_url,
            timeout=60.0,
        )

        full_reply = ""
        tool_rounds = 0

        while tool_rounds <= self.max_tool_rounds:
            try:
                call_kwargs: dict[str, Any] = {
                    "model": config.chat_model,
                    "messages": messages,
                    "temperature": 0.3,
                    "stream": True,
                }
                if tools_schema and tool_rounds < self.max_tool_rounds:
                    call_kwargs["tools"] = tools_schema
                    call_kwargs["tool_choice"] = "auto"

                stream = await client.chat.completions.create(**call_kwargs)

                current_content = ""
                tool_calls_buffer: list[dict[str, Any]] = []

                async for chunk in stream:
                    delta = chunk.choices[0].delta if chunk.choices else None
                    if not delta:
                        continue

                    # 文本内容 — 流式输出
                    if delta.content:
                        current_content += delta.content
                        yield self._event("text", content=delta.content)

                    # Tool calls 收集
                    if delta.tool_calls:
                        for tc in delta.tool_calls:
                            if tc.index is not None:
                                while len(tool_calls_buffer) <= tc.index:
                                    tool_calls_buffer.append(
                                        {"id": "", "function": {"name": "", "arguments": ""}}
                                    )
                                entry = tool_calls_buffer[tc.index]
                                if tc.id:
                                    entry["id"] = tc.id
                                if tc.function:
                                    if tc.function.name:
                                        entry["function"]["name"] += tc.function.name
                                    if tc.function.arguments:
                                        entry["function"]["arguments"] += tc.function.arguments

                # 有文本且无 tool_calls → 结束
                if current_content and not tool_calls_buffer:
                    full_reply = current_content
                    break

                # 有 tool_calls → 执行工具并继续
                if tool_calls_buffer:
                    tool_rounds += 1
                    yield self._event("thinking", content=f"正在调用 {len(tool_calls_buffer)} 个工具...")
                    await asyncio.sleep(0.7)

                    assistant_msg: dict[str, Any] = {
                        "role": "assistant",
                        "content": current_content or None,
                    }
                    assistant_msg["tool_calls"] = [
                        {
                            "id": tc["id"],
                            "type": "function",
                            "function": {
                                "name": tc["function"]["name"],
                                "arguments": tc["function"]["arguments"],
                            },
                        }
                        for tc in tool_calls_buffer
                    ]
                    messages.append(assistant_msg)

                    # 执行每个 tool call 并发送事件
                    for tc in tool_calls_buffer:
                        tool_name = tc["function"]["name"]
                        try:
                            raw_args = json.loads(tc["function"]["arguments"])
                        except json.JSONDecodeError:
                            raw_args = {}

                        # 发送 tool_call 事件（前端先渲染"调用中"卡片）
                        yield self._event(
                            "tool_call", name=tool_name, args=raw_args
                        )

                        # 执行工具（内含 0.8s 延迟）
                        result = await self._execute_tool(
                            tool_name, raw_args, tool_ctx, db
                        )

                        # 发送 tool_result 事件
                        yield self._event(
                            "tool_result",
                            name=tool_name,
                            success=result.get("status") == "success",
                            data=result.get("data"),
                            error=result.get("error"),
                        )

                        messages.append({
                            "role": "tool",
                            "tool_call_id": tc["id"],
                            "content": json.dumps(result, ensure_ascii=False),
                        })

                    yield self._event("thinking", content="正在基于查询结果生成回复...")
                    await asyncio.sleep(0.8)
                    continue
                else:
                    if not current_content:
                        full_reply = "抱歉，AI 未能生成回复。"
                        yield self._event("text", content=full_reply)
                    else:
                        full_reply = current_content
                    break

            except Exception as e:  # noqa: BLE001
                log.warning("chat_agent.stream_error", error=str(e))
                error_msg = f"AI 服务异常：{type(e).__name__}: {e}"
                yield self._event("error", message=error_msg)
                full_reply += f"\n\n[{error_msg}]"
                break

        # 6. 持久化 assistant 回复
        db.add(ChatMessage(
            session_id=session.id,
            role="assistant",
            content=full_reply,
        ))
        await db.flush()

        # 7. 发送完成事件
        yield self._event("done", full_content=full_reply)

    @staticmethod
    def _event(event_type: str, **kwargs: Any) -> str:
        """构建 SSE 事件 JSON."""
        payload: dict[str, Any] = {"type": event_type}
        payload.update(kwargs)
        return json.dumps(payload, ensure_ascii=False)

    async def _build_system_prompt(
        self, db: AsyncSession, user: User, session: ChatSession
    ) -> str:
        """根据角色构建 system prompt，注入用户上下文."""
        if user.role == "teacher":
            ctx = await _build_teacher_context(db, user)
            return _TEACHER_SYSTEM_PROMPT.format(user_context=ctx)
        elif user.role == "admin":
            ctx = await _build_admin_context(db, user)
            return _ADMIN_SYSTEM_PROMPT.format(user_context=ctx)
        else:
            ctx = await _build_student_context(db, user)
            # 如果会话关联了评价，追加评价上下文
            if session.evaluation_id:
                eval_ctx = await self._build_evaluation_context(
                    db, session.evaluation_id
                )
                ctx += f"\n\n## 当前关联评价\n{eval_ctx}"
            return _STUDENT_SYSTEM_PROMPT.format(user_context=ctx)

    async def _build_evaluation_context(
        self, db: AsyncSession, evaluation_id: int
    ) -> str:
        """构建评价上下文（当会话关联了某次评价时）."""
        from app.models.evaluation import DimensionScore, Evaluation
        from app.models.task import Dimension, TrainingTask

        ev = await db.get(Evaluation, evaluation_id)
        if not ev:
            return "评价不存在"

        task = await db.get(TrainingTask, ev.task_id)
        lines = [
            f"- 评价 ID：{ev.id}",
            f"- 任务：{task.name if task else '未知'}",
            f"- 综合得分：{ev.total_score}",
            f"- 状态：{ev.status}",
        ]

        # 各维度得分
        scores = list(
            (
                await db.execute(
                    select(DimensionScore, Dimension.name)
                    .join(Dimension, Dimension.id == DimensionScore.dimension_id)
                    .where(DimensionScore.evaluation_id == evaluation_id)
                )
            ).all()
        )
        if scores:
            lines.append("- 维度得分：")
            for ds, dim_name in scores:
                ai = ds.ai_score if ds.ai_score is not None else "—"
                teacher = ds.teacher_score if ds.teacher_score is not None else "—"
                lines.append(f"  · {dim_name}: AI={ai}, 教师={teacher}")
                if ds.rationale:
                    lines.append(f"    理由：{ds.rationale[:100]}")

        return "\n".join(lines)

    async def _load_history(
        self, db: AsyncSession, session_id: int
    ) -> list[dict[str, str]]:
        """加载对话历史（最近 N 条），转为 OpenAI messages 格式."""
        msgs = list(
            (
                await db.execute(
                    select(ChatMessage)
                    .where(ChatMessage.session_id == session_id)
                    .order_by(ChatMessage.created_at.desc())
                    .limit(MAX_HISTORY_MESSAGES)
                )
            )
            .scalars()
            .all()
        )
        # 反转为时间正序
        msgs.reverse()
        return [
            {"role": m.role, "content": m.content}
            for m in msgs
            if m.role in ("user", "assistant")
        ]

    async def _load_history_with_summary(
        self, db: AsyncSession, session_id: int
    ) -> list[dict[str, str]]:
        """加载对话历史，超过阈值时压缩前文为摘要."""
        all_msgs = list(
            (
                await db.execute(
                    select(ChatMessage)
                    .where(ChatMessage.session_id == session_id)
                    .order_by(ChatMessage.created_at.asc())
                )
            )
            .scalars()
            .all()
        )
        relevant = [m for m in all_msgs if m.role in ("user", "assistant")]

        if len(relevant) <= SUMMARY_THRESHOLD:
            return [{"role": m.role, "content": m.content} for m in relevant]

        # 压缩前面的消息为摘要，保留最近 6 条原文
        keep_recent = 6
        old_msgs = relevant[:-keep_recent]
        recent_msgs = relevant[-keep_recent:]

        summary_parts: list[str] = []
        for m in old_msgs[-8:]:
            prefix = "用户" if m.role == "user" else "AI"
            summary_parts.append(f"{prefix}: {m.content[:80]}")

        summary_text = (
            "[以下是之前对话的摘要]\n"
            + "\n".join(summary_parts)
            + "\n[摘要结束，以下是最近对话]"
        )

        result: list[dict[str, str]] = [
            {"role": "system", "content": summary_text},
        ]
        result.extend(
            {"role": m.role, "content": m.content} for m in recent_msgs
        )
        return result

    def _get_tools_for_role(self, role: str) -> list[dict[str, Any]]:
        """根据角色返回可用工具的 OpenAI schema."""
        from app.llm.tools.chat_tools import CHAT_TOOLS

        # 确保工具已注册
        for tool_cls in CHAT_TOOLS:
            if tool_cls.name not in tool_registry.list_names():
                tool_registry.register(tool_cls)

        # 过滤角色可用的工具
        schemas: list[dict[str, Any]] = []
        for name in tool_registry.list_names():
            tool_instance = tool_registry.get(name)
            if not tool_instance.allowed_roles or role in tool_instance.allowed_roles:
                schemas.append(tool_instance.to_openai_schema())
        return schemas

    async def _execute_tool(
        self,
        tool_name: str,
        raw_args: dict[str, Any],
        ctx: ToolExecutionContext,
        db: AsyncSession | None = None,
    ) -> dict[str, Any]:
        """执行工具调用，返回结果字典."""
        import asyncio

        try:
            tool_instance = tool_registry.get(tool_name)
            # 演示级延迟 1.5-2s：让前端充分展示工具调用动画，体现 Agent 推理过程
            await asyncio.sleep(1.5 + (hash(tool_name) % 5) * 0.1)
            result: ToolResult = await tool_instance(raw_args, ctx)
            if result.success:
                return {"status": "success", "data": result.data}
            else:
                return {"status": "error", "error": result.error or "未知错误"}
        except KeyError:
            return {"status": "error", "error": f"工具 {tool_name} 不存在"}
        except Exception as e:  # noqa: BLE001
            log.warning("chat_agent.tool_error", tool=tool_name, error=str(e))
            return {"status": "error", "error": str(e)}
