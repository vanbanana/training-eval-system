"""PDF 报告渲染（Epic 20.2）.

使用 reportlab 直接绘制 PDF，避免 WeasyPrint 在 LoongArch 上的依赖问题。
作为兜底，也保留 HTML 渲染（无 PDF 依赖时返回 HTML 字节）。
"""

from __future__ import annotations

import html
import io
from typing import Any


def render_personal_html(data: dict[str, Any]) -> str:
    """生成可直接打印的 HTML 字符串（不依赖任何 PDF 库）."""
    student = data.get("student_name", "")
    task = data.get("task_name", "")
    total = data.get("total_score", "")
    dims = data.get("dimensions", [])
    comment = data.get("teacher_comment", "")

    rows = "".join(
        f'<tr><td>{html.escape(d.get("name", ""))}</td>'
        f'<td>{d.get("ai_score", "")}</td>'
        f'<td>{d.get("teacher_score", "")}</td>'
        f'<td>{html.escape(d.get("rationale", ""))}</td></tr>'
        for d in dims
    )
    return (
        '<!DOCTYPE html>'
        '<html><head><meta charset="utf-8">'
        '<style>'
        'body{font-family:"Noto Sans CJK SC",sans-serif;margin:24px}'
        'h1{margin:0 0 12px}'
        '.meta{color:#475569;margin-bottom:16px}'
        'table{border-collapse:collapse;width:100%}'
        'th,td{border:1px solid #cbd5e1;padding:8px;text-align:left}'
        'th{background:#f1f5f9}'
        '.score{font-size:32px;color:#0ea5e9}'
        '.section{margin-top:20px}'
        '</style></head><body>'
        f'<h1>个人评价报告</h1>'
        f'<div class="meta">学生：{html.escape(str(student))} ｜ 任务：{html.escape(str(task))}</div>'
        f'<div class="section">综合得分：<span class="score">{total}</span></div>'
        f'<div class="section">'
        f'<table><thead><tr><th>维度</th><th>客观分</th><th>主观分</th><th>评分理由</th></tr></thead>'
        f'<tbody>{rows}</tbody></table></div>'
        f'<div class="section"><strong>教师评语：</strong>{html.escape(str(comment))}</div>'
        '</body></html>'
    )


def render_personal_pdf(data: dict[str, Any]) -> bytes:
    """优先使用 reportlab 输出 PDF；不可用时输出 UTF-8 编码的 HTML."""
    try:
        from reportlab.lib.pagesizes import A4  # type: ignore[import-not-found]
        from reportlab.pdfbase import pdfmetrics  # type: ignore[import-not-found]
        from reportlab.pdfbase.ttfonts import TTFont  # type: ignore[import-not-found]
        from reportlab.pdfgen import canvas  # type: ignore[import-not-found]
    except ImportError:
        return render_personal_html(data).encode("utf-8")

    buf = io.BytesIO()
    c = canvas.Canvas(buf, pagesize=A4)
    width, height = A4
    # 使用默认字体（中文需在生产部署时挂载字体；这里兜底为 Helvetica）
    try:
        c.setFont("Helvetica-Bold", 16)
    except Exception:  # noqa: BLE001
        pass
    c.drawString(40, height - 50, "Personal Evaluation Report")
    c.setFont("Helvetica", 11)
    c.drawString(40, height - 80, f"Student: {data.get('student_name', '')}")
    c.drawString(40, height - 100, f"Task: {data.get('task_name', '')}")
    c.drawString(40, height - 120, f"Total Score: {data.get('total_score', '')}")

    y = height - 160
    c.setFont("Helvetica-Bold", 12)
    c.drawString(40, y, "Dimensions")
    y -= 18
    c.setFont("Helvetica", 10)
    for d in data.get("dimensions", []):
        c.drawString(
            40,
            y,
            f"- {d.get('name', '')}: ai={d.get('ai_score')}, "
            f"teacher={d.get('teacher_score')}",
        )
        y -= 14
        if y < 60:
            c.showPage()
            y = height - 50

    y -= 20
    c.setFont("Helvetica-Bold", 12)
    c.drawString(40, y, "Teacher comment")
    y -= 16
    c.setFont("Helvetica", 10)
    c.drawString(40, y, str(data.get("teacher_comment", "")))

    c.showPage()
    c.save()
    return buf.getvalue()


__all__ = ["render_personal_html", "render_personal_pdf"]
