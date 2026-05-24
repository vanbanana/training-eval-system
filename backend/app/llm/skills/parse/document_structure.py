"""parse.document_structure Skill - 调用 LLM 对解析后的原始文本生成结构化摘要.

输入：原始文本 + 文件类型 + 任务要求（可选）
输出：结构化摘要（主题列表、关键发现、技术栈、完成度评估）
"""

from __future__ import annotations

from typing import ClassVar

from pydantic import BaseModel, Field

from app.llm.base import LLMMessage
from app.llm.skills.base import Skill


class DocumentSection(BaseModel):
    """文档中识别出的一个逻辑段落/章节."""

    title: str = Field(..., description="段落/章节标题")
    summary: str = Field(..., description="内容摘要（50-200字）")
    key_points: list[str] = Field(default_factory=list, description="关键要点")


class DocumentStructureInput(BaseModel):
    """结构化解析输入."""

    raw_text: str = Field(..., description="解析器提取的原始文本（截断至前 5000 字）")
    file_type: str = Field(..., description="文件类型: docx/pdf/xlsx/zip/png/jpg")
    task_requirements: str = Field(default="", description="实训任务要求（可选，用于上下文）")
    filename: str = Field(default="", description="原始文件名")


class DocumentStructureOutput(BaseModel):
    """结构化解析输出."""

    summary: str = Field(..., description="全文摘要（100-300字）")
    sections: list[DocumentSection] = Field(
        default_factory=list, description="识别出的逻辑章节"
    )
    key_topics: list[str] = Field(
        default_factory=list, description="涉及的核心主题/技术点"
    )
    completeness_assessment: str = Field(
        default="", description="完成度初步评估（一句话）"
    )
    word_count: int = Field(default=0, description="估算字数")
    has_code: bool = Field(default=False, description="是否包含代码")
    has_diagrams: bool = Field(default=False, description="是否包含图表描述")


class DocumentStructureSkill(Skill[DocumentStructureInput, DocumentStructureOutput]):
    """文档结构化解析 Skill.

    调用 LLM 对解析器提取的原始文本进行智能结构化，
    生成摘要、章节划分、关键主题等信息。
    """

    name: ClassVar[str] = "parse.document_structure"
    version: ClassVar[str] = "1.0.0"
    category: ClassVar[str] = "parse"
    temperature: ClassVar[float] = 0.1
    max_tokens: ClassVar[int | None] = 2000
    input_schema: ClassVar[type[BaseModel]] = DocumentStructureInput
    output_schema: ClassVar[type[BaseModel]] = DocumentStructureOutput

    def render_prompt(self, input_: DocumentStructureInput) -> list[LLMMessage]:
        file_type_desc = {
            "docx": "Word 文档",
            "pdf": "PDF 文档",
            "xlsx": "Excel 表格",
            "zip": "源代码压缩包",
            "png": "图片（OCR 识别）",
            "jpg": "图片（OCR 识别）",
            "jpeg": "图片（OCR 识别）",
        }.get(input_.file_type, "未知类型")

        system_prompt = (
            "你是一个专业的实训成果分析助手。你的任务是对学生提交的实训成果进行结构化分析。\n"
            "请严格按照 JSON 格式输出，不要包含任何额外文本或 markdown 标记。\n\n"
            "输出格式要求：\n"
            "{\n"
            '  "summary": "全文摘要（100-300字，概括主要内容和完成情况）",\n'
            '  "sections": [\n'
            '    {"title": "章节标题", "summary": "内容摘要", "key_points": ["要点1", "要点2"]}\n'
            "  ],\n"
            '  "key_topics": ["核心主题1", "核心主题2"],\n'
            '  "completeness_assessment": "完成度一句话评估",\n'
            '  "word_count": 估算字数(整数),\n'
            '  "has_code": true/false,\n'
            '  "has_diagrams": true/false\n'
            "}\n\n"
            "注意事项：\n"
            "- sections 最多 10 个，按文档顺序排列\n"
            "- key_topics 最多 8 个\n"
            "- 如果是源代码，关注代码结构、使用的技术栈、功能实现\n"
            "- 如果是文档，关注章节结构、论述逻辑、图表引用\n"
            "- 如果是表格，关注数据组织方式和关键数据\n"
        )

        user_parts = [
            f"## 文件信息\n- 类型: {file_type_desc}\n- 文件名: {input_.filename or '未知'}",
        ]

        if input_.task_requirements:
            user_parts.append(
                f"\n## 实训任务要求（参考上下文）\n{input_.task_requirements[:1000]}"
            )

        # 截断原始文本，避免超出 token 限制
        text = input_.raw_text[:5000]
        if len(input_.raw_text) > 5000:
            text += "\n\n[... 内容已截断，以上为前 5000 字 ...]"

        user_parts.append(f"\n## 提交内容\n{text}")
        user_parts.append("\n请分析以上内容并输出 JSON。")

        return [
            LLMMessage(role="system", content=system_prompt),
            LLMMessage(role="user", content="\n".join(user_parts)),
        ]
