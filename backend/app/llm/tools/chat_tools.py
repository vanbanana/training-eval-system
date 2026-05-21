"""Chat Tools - 真实数据版.

7 个学生工具，全部查询真实 DB 数据。
通过 ctx.db（AsyncSession）访问数据库。
通过 ctx.actor_id / ctx.student_id 限制数据范围（只能查自己的）。
"""

from __future__ import annotations

from typing import Any, ClassVar

from pydantic import BaseModel, Field
from sqlalchemy import func as sqlfunc
from sqlalchemy import select

from app.llm.tools.base import (
    Tool,
    ToolExecutionContext,
    ToolPermissionError,
    ToolResult,
)


# ---- Input Schemas ----

class ParseSegmentArgs(BaseModel):
    upload_id: int
    keyword: str = ""


class DimensionDetailArgs(BaseModel):
    evaluation_id: int
    dimension_id: int


class ClassStatisticsArgs(BaseModel):
    task_id: int


class DimensionHistoryArgs(BaseModel):
    dimension_name: str
    last_n: int = Field(default=5, ge=1, le=20)

class ExcellentSampleArgs(BaseModel):
    task_id: int


class WeaknessListArgs(BaseModel):
    last_n: int = Field(default=5, ge=1, le=20)


class LearningResourcesArgs(BaseModel):
    dimension_name: str


# ---- Tool Implementations ----


class GetParseSegmentTool(Tool[ParseSegmentArgs]):
    """查询学生提交的解析内容片段."""

    name: ClassVar[str] = "get_parse_segment"
    description: ClassVar[str] = "查询当前学生某次提交的解析文本片段（可按关键词过滤）"
    input_schema: ClassVar[type[BaseModel]] = ParseSegmentArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: ParseSegmentArgs, ctx: ToolExecutionContext) -> ToolResult:
        from app.models.upload import ParseResult, Upload

        if ctx.evaluation_id is None:
            raise ToolPermissionError("工具需要 evaluation_id 上下文", field="ctx.evaluation_id")

        db = ctx.db
        if db is None:
            return ToolResult(success=True, data={"snippet": "（数据查询不可用）", "upload_id": args.upload_id})

        upload = await db.get(Upload, args.upload_id)
        if not upload:
            return ToolResult(success=False, error=f"上传 {args.upload_id} 不存在")
        if upload.student_id != ctx.actor_id:
            raise ToolPermissionError("无权查看他人提交", field="upload_id")

        pr = (await db.execute(
            select(ParseResult).where(ParseResult.upload_id == args.upload_id)
        )).scalar_one_or_none()

        if not pr or not pr.raw_text:
            return ToolResult(success=True, data={"snippet": "（该提交尚未完成解析）", "upload_id": args.upload_id})

        text = pr.raw_text
        if args.keyword:
            # 找到关键词附近 200 字
            idx = text.find(args.keyword)
            if idx >= 0:
                start = max(0, idx - 100)
                end = min(len(text), idx + len(args.keyword) + 100)
                text = "..." + text[start:end] + "..."
            else:
                text = f"（未找到关键词「{args.keyword}」）"
        else:
            text = text[:300] + ("..." if len(text) > 300 else "")

        return ToolResult(success=True, data={"upload_id": args.upload_id, "snippet": text})


class GetDimensionDetailTool(Tool[DimensionDetailArgs]):
    """查询某次评价中某个维度的详细评分."""

    name: ClassVar[str] = "get_dimension_detail"
    description: ClassVar[str] = "查询某次评价中某个维度的 AI 评分、教师评分和评分理由"
    input_schema: ClassVar[type[BaseModel]] = DimensionDetailArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: DimensionDetailArgs, ctx: ToolExecutionContext) -> ToolResult:
        from app.models.evaluation import DimensionScore, Evaluation
        from app.models.task import Dimension

        # 越权校验：只能查自己关联的评价
        if ctx.evaluation_id is not None and args.evaluation_id != ctx.evaluation_id:
            raise ToolPermissionError("evaluation_id 越权", field="evaluation_id")

        db = ctx.db
        if db is None:
            return ToolResult(success=True, data={
                "evaluation_id": args.evaluation_id,
                "dimension_id": args.dimension_id,
                "message": "数据查询不可用",
            })

        ev = await db.get(Evaluation, args.evaluation_id)
        if not ev:
            return ToolResult(success=False, error=f"评价 {args.evaluation_id} 不存在")
        if ev.student_id != ctx.actor_id:
            raise ToolPermissionError("无权查看他人评价", field="evaluation_id")

        ds = (await db.execute(
            select(DimensionScore).where(
                DimensionScore.evaluation_id == args.evaluation_id,
                DimensionScore.dimension_id == args.dimension_id,
            )
        )).scalar_one_or_none()

        if not ds:
            return ToolResult(success=False, error=f"维度 {args.dimension_id} 无评分记录")

        dim = await db.get(Dimension, args.dimension_id)
        return ToolResult(success=True, data={
            "dimension_name": dim.name if dim else f"维度#{args.dimension_id}",
            "ai_score": ds.ai_score,
            "teacher_score": ds.teacher_score,
            "rationale": ds.rationale,
            "weight": dim.weight if dim else None,
        })


class GetClassStatisticsTool(Tool[ClassStatisticsArgs]):
    """查询某任务下班级整体统计（脱敏）."""

    name: ClassVar[str] = "get_class_statistics"
    description: ClassVar[str] = "查询某任务下班级整体评分统计（平均分、中位数、四分位数），不含个人信息"
    input_schema: ClassVar[type[BaseModel]] = ClassStatisticsArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: ClassStatisticsArgs, ctx: ToolExecutionContext) -> ToolResult:
        from app.models.evaluation import Evaluation

        db = ctx.db
        if db is None:
            # 无 DB 时返回空统计（兼容单元测试）
            return ToolResult(success=True, data={
                "task_id": args.task_id,
                "message": "数据查询不可用",
                "count": 0,
            })

        scores = list(
            (await db.execute(
                select(Evaluation.total_score).where(
                    Evaluation.task_id == args.task_id,
                    Evaluation.total_score.isnot(None),
                )
            )).scalars()
        )

        if not scores:
            return ToolResult(success=True, data={
                "task_id": args.task_id,
                "message": "该任务暂无评价数据",
                "count": 0,
            })

        scores_sorted = sorted(scores)
        n = len(scores_sorted)
        avg = round(sum(scores_sorted) / n, 1)
        median = round(scores_sorted[n // 2], 1)
        p25 = round(scores_sorted[n // 4], 1)
        p75 = round(scores_sorted[3 * n // 4], 1)
        max_score = round(max(scores_sorted), 1)
        min_score = round(min(scores_sorted), 1)

        return ToolResult(success=True, data={
            "task_id": args.task_id,
            "count": n,
            "avg": avg,
            "median": median,
            "p25": p25,
            "p75": p75,
            "max": max_score,
            "min": min_score,
        })


class GetDimensionHistoryTool(Tool[DimensionHistoryArgs]):
    """查询当前学生在某维度的历史评分曲线."""

    name: ClassVar[str] = "get_dimension_history"
    description: ClassVar[str] = "查询当前学生在指定维度名称的历史评分列表（按时间排序）"
    input_schema: ClassVar[type[BaseModel]] = DimensionHistoryArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: DimensionHistoryArgs, ctx: ToolExecutionContext) -> ToolResult:
        from app.models.evaluation import DimensionScore, Evaluation
        from app.models.task import Dimension

        db = ctx.db
        if db is None:
            return ToolResult(success=True, data={"message": "数据查询不可用"})

        rows = list(
            (await db.execute(
                select(DimensionScore.ai_score, DimensionScore.teacher_score, Evaluation.created_at)
                .join(Evaluation, Evaluation.id == DimensionScore.evaluation_id)
                .join(Dimension, Dimension.id == DimensionScore.dimension_id)
                .where(
                    Evaluation.student_id == ctx.actor_id,
                    Dimension.name == args.dimension_name,
                    DimensionScore.ai_score.isnot(None),
                )
                .order_by(Evaluation.created_at.desc())
                .limit(args.last_n)
            )).all()
        )

        if not rows:
            return ToolResult(success=True, data={
                "dimension_name": args.dimension_name,
                "message": f"未找到维度「{args.dimension_name}」的历史评分",
                "scores": [],
            })

        # 反转为时间正序
        rows.reverse()
        history = [
            {
                "ai_score": round(float(r[0]), 1) if r[0] else None,
                "teacher_score": round(float(r[1]), 1) if r[1] else None,
                "date": r[2].strftime("%m-%d") if r[2] else "",
            }
            for r in rows
        ]

        return ToolResult(success=True, data={
            "dimension_name": args.dimension_name,
            "count": len(history),
            "scores": history,
        })


class GetExcellentSampleSummaryTool(Tool[ExcellentSampleArgs]):
    """查询某任务下高分样本的脱敏摘要."""

    name: ClassVar[str] = "get_excellent_sample_summary"
    description: ClassVar[str] = "查询某任务下高分提交的脱敏摘要（不含学生姓名），帮助学生了解优秀作品特点"
    input_schema: ClassVar[type[BaseModel]] = ExcellentSampleArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: ExcellentSampleArgs, ctx: ToolExecutionContext) -> ToolResult:
        from app.models.evaluation import Evaluation
        from app.models.upload import ParseResult, Upload

        db = ctx.db
        if db is None:
            return ToolResult(success=True, data={"message": "数据查询不可用"})

        # 找该任务下得分最高的 3 个评价对应的上传
        top_evals = list(
            (await db.execute(
                select(Evaluation)
                .where(
                    Evaluation.task_id == args.task_id,
                    Evaluation.total_score.isnot(None),
                    Evaluation.student_id != ctx.actor_id,  # 排除自己
                )
                .order_by(Evaluation.total_score.desc())
                .limit(3)
            )).scalars()
        )

        if not top_evals:
            return ToolResult(success=True, data={
                "task_id": args.task_id,
                "message": "该任务暂无其他同学的评价数据",
                "summaries": [],
            })

        summaries: list[dict[str, Any]] = []
        for ev in top_evals:
            # 获取解析文本摘要
            pr = (await db.execute(
                select(ParseResult)
                .join(Upload, Upload.id == ParseResult.upload_id)
                .where(Upload.task_id == args.task_id, Upload.student_id == ev.student_id)
            )).scalar_one_or_none()

            snippet = ""
            if pr and pr.raw_text:
                snippet = pr.raw_text[:150] + "..."

            summaries.append({
                "score": round(float(ev.total_score), 1) if ev.total_score else 0,
                "snippet": snippet or "（解析内容不可用）",
            })

        return ToolResult(success=True, data={
            "task_id": args.task_id,
            "count": len(summaries),
            "summaries": summaries,
        })


class GetWeaknessListTool(Tool[WeaknessListArgs]):
    """查询当前学生的薄弱维度列表."""

    name: ClassVar[str] = "get_weakness_list"
    description: ClassVar[str] = "查询当前学生所有维度的平均分，按分数升序排列（薄弱点在前）"
    input_schema: ClassVar[type[BaseModel]] = WeaknessListArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: WeaknessListArgs, ctx: ToolExecutionContext) -> ToolResult:
        from app.models.evaluation import DimensionScore, Evaluation
        from app.models.task import Dimension

        db = ctx.db
        if db is None:
            return ToolResult(success=True, data={"message": "数据查询不可用"})

        rows = list(
            (await db.execute(
                select(Dimension.name, sqlfunc.avg(DimensionScore.ai_score), sqlfunc.count(DimensionScore.id))
                .join(DimensionScore, DimensionScore.dimension_id == Dimension.id)
                .join(Evaluation, Evaluation.id == DimensionScore.evaluation_id)
                .where(
                    Evaluation.student_id == ctx.actor_id,
                    DimensionScore.ai_score.isnot(None),
                )
                .group_by(Dimension.name)
                .order_by(sqlfunc.avg(DimensionScore.ai_score).asc())
                .limit(args.last_n)
            )).all()
        )

        if not rows:
            return ToolResult(success=True, data={
                "message": "暂无评价数据，无法分析薄弱点",
                "weaknesses": [],
            })

        weaknesses = [
            {
                "name": name,
                "avg_score": round(float(avg), 1) if avg else 0,
                "eval_count": int(count),
            }
            for name, avg, count in rows
        ]

        return ToolResult(success=True, data={
            "count": len(weaknesses),
            "weaknesses": weaknesses,
        })


class GetLearningResourcesTool(Tool[LearningResourcesArgs]):
    """根据维度推荐学习资源."""

    name: ClassVar[str] = "get_learning_resources"
    description: ClassVar[str] = "根据薄弱维度名称，推荐针对性的学习资源和改进建议"
    input_schema: ClassVar[type[BaseModel]] = LearningResourcesArgs
    allowed_roles: ClassVar[set[str]] = {"student"}

    async def execute(self, args: LearningResourcesArgs, ctx: ToolExecutionContext) -> ToolResult:
        # 学习资源是基于维度名称的知识库映射（不需要 DB 查询）
        # 实际生产中可以接入知识库或让 LLM 自己生成
        resources_map: dict[str, list[dict[str, str]]] = {
            "代码规范": [
                {"title": "Google Java Style Guide", "url": "https://google.github.io/styleguide/javaguide.html"},
                {"title": "Clean Code 要点总结", "url": "https://gist.github.com/wojteklu/73c6914cc446146b8b533c0988cf8d29"},
            ],
            "功能实现": [
                {"title": "设计模式精要", "url": "https://refactoring.guru/design-patterns"},
                {"title": "算法可视化", "url": "https://visualgo.net/zh"},
            ],
            "测试验证": [
                {"title": "JUnit 5 用户指南", "url": "https://junit.org/junit5/docs/current/user-guide/"},
                {"title": "测试驱动开发入门", "url": "https://www.guru99.com/test-driven-development.html"},
            ],
            "文档规范": [
                {"title": "技术写作指南", "url": "https://developers.google.com/tech-writing"},
                {"title": "Markdown 语法速查", "url": "https://www.markdownguide.org/cheat-sheet/"},
            ],
        }

        # 模糊匹配维度名
        matched: list[dict[str, str]] = []
        for key, resources in resources_map.items():
            if key in args.dimension_name or args.dimension_name in key:
                matched = resources
                break

        if not matched:
            # 通用推荐
            matched = [
                {"title": f"「{args.dimension_name}」相关学习资料", "url": "https://www.bilibili.com/search?keyword=" + args.dimension_name},
                {"title": "向 AI 助手追问具体改进方法", "url": ""},
            ]

        return ToolResult(success=True, data={
            "dimension_name": args.dimension_name,
            "resources": matched,
            "tip": f"建议重点关注「{args.dimension_name}」维度，结合以上资源进行针对性练习",
        })


# ---- 导出 ----

CHAT_TOOLS: list[type[Tool]] = [
    GetParseSegmentTool,
    GetDimensionDetailTool,
    GetClassStatisticsTool,
    GetDimensionHistoryTool,
    GetExcellentSampleSummaryTool,
    GetWeaknessListTool,
    GetLearningResourcesTool,
]

__all__ = [
    "CHAT_TOOLS",
    "GetClassStatisticsTool",
    "GetDimensionDetailTool",
    "GetDimensionHistoryTool",
    "GetExcellentSampleSummaryTool",
    "GetLearningResourcesTool",
    "GetParseSegmentTool",
    "GetWeaknessListTool",
]
