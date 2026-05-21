"""NotificationFactory - Epic 21.1."""

from __future__ import annotations

import random
from typing import Any

from app.models.notification import Notification
from app.services.notification_events import (
    DEADLINE_APPROACHING,
    EVALUATION_COMPLETED,
    PARSE_COMPLETED,
    SIMILARITY_DETECTED,
    TASK_PUBLISHED,
    UPLOAD_REJECTED,
)
from sqlalchemy.ext.asyncio import AsyncSession

# 不同事件的标题模板（贴近真实使用，避免占位符感）
_TITLE_TEMPLATES: dict[str, str] = {
    TASK_PUBLISHED: "新任务已发布：{task_name}",
    PARSE_COMPLETED: "你的提交已完成解析",
    EVALUATION_COMPLETED: "评价已生成：得分 {score}",
    UPLOAD_REJECTED: "提交被退回：{reason}",
    SIMILARITY_DETECTED: "提交相似度告警",
    DEADLINE_APPROACHING: "任务即将截止：{task_name}",
}

_CONTENT_TEMPLATES: dict[str, str] = {
    TASK_PUBLISHED: "请在截止时间前提交。",
    PARSE_COMPLETED: "AI 已完成材料解析，等待评分。",
    EVALUATION_COMPLETED: "查看完整评价报告以了解每个维度的细节。",
    UPLOAD_REJECTED: "请根据老师反馈修改后重新提交。",
    SIMILARITY_DETECTED: "系统检测到与同班级提交存在较高相似度，正在复核。",
    DEADLINE_APPROACHING: "请尽快完成提交。",
}


class NotificationFactory:
    """注入一条通知（不走 NotificationService 偏好过滤）."""

    @classmethod
    async def create_async(
        cls,
        session: AsyncSession,
        *,
        user_id: int,
        event_type: str = EVALUATION_COMPLETED,
        title: str | None = None,
        content: str | None = None,
        is_read: bool = False,
        link: str = "",
        payload: dict[str, Any] | None = None,
        rng: random.Random | None = None,
        **template_vars: Any,
    ) -> Notification:
        rnd = rng or random
        if title is None:
            tpl = _TITLE_TEMPLATES.get(event_type, "系统通知")
            try:
                title = tpl.format(
                    task_name=template_vars.get("task_name", "实训任务"),
                    score=template_vars.get(
                        "score", round(rnd.uniform(60, 95), 1)
                    ),
                    reason=template_vars.get("reason", "材料不完整"),
                )
            except KeyError:
                title = tpl
        if content is None:
            content = _CONTENT_TEMPLATES.get(event_type, "")
        n = Notification(
            user_id=user_id,
            type=event_type,
            title=title,
            content=content,
            is_read=is_read,
            link=link,
            payload=payload,
        )
        session.add(n)
        await session.flush()
        await session.refresh(n)
        return n
