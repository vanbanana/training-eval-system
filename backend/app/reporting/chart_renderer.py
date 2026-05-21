"""图表渲染（Epic 20.1）.

使用纯 Python 生成 SVG，避免 LoongArch 上 matplotlib 安装麻烦。
SVG 可直接嵌入 HTML/PDF，且 vector 图形无 DPI 困扰。
中文用通用字体族避免缺字。
"""

from __future__ import annotations

import math
from typing import Any


_FONT = "Noto Sans CJK SC, Microsoft YaHei, PingFang SC, sans-serif"


def render_radar_chart_svg(
    labels: list[str],
    values: list[float],
    *,
    width: int = 360,
    height: int = 360,
    max_value: float = 100.0,
) -> str:
    """渲染雷达图为 SVG 字符串."""
    if not labels or len(labels) != len(values):
        raise ValueError("labels 与 values 长度必须一致")
    cx, cy = width / 2, height / 2
    radius = min(cx, cy) * 0.75
    n = len(labels)

    def _coord(i: int, value: float) -> tuple[float, float]:
        angle = -math.pi / 2 + i * 2 * math.pi / n
        r = radius * (value / max_value)
        return cx + r * math.cos(angle), cy + r * math.sin(angle)

    pts = " ".join(f"{x:.1f},{y:.1f}" for i, v in enumerate(values) for x, y in [_coord(i, v)])

    # 网格
    grid = []
    for level in (0.25, 0.5, 0.75, 1.0):
        ring_pts = " ".join(
            f"{x:.1f},{y:.1f}"
            for i in range(n)
            for x, y in [_coord(i, max_value * level)]
        )
        grid.append(
            f'<polygon points="{ring_pts}" fill="none" stroke="#e2e8f0" />'
        )
    # 文字标签
    text_elems = []
    for i, label in enumerate(labels):
        x, y = _coord(i, max_value * 1.12)
        anchor = "middle"
        if x < cx - 5:
            anchor = "end"
        elif x > cx + 5:
            anchor = "start"
        text_elems.append(
            f'<text x="{x:.1f}" y="{y:.1f}" text-anchor="{anchor}" '
            f'font-size="12" font-family="{_FONT}" fill="#0f172a">{label}</text>'
        )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}">'
        f'{"".join(grid)}'
        f'<polygon points="{pts}" fill="rgba(59,130,246,0.25)" stroke="#3b82f6" stroke-width="2" />'
        f'{"".join(text_elems)}'
        f'</svg>'
    )


def render_bar_chart_svg(
    labels: list[str],
    values: list[float],
    *,
    width: int = 480,
    height: int = 240,
    max_value: float = 100.0,
) -> str:
    if not labels or len(labels) != len(values):
        raise ValueError("labels 与 values 长度必须一致")
    n = len(values)
    pad = 36
    bar_area_w = width - pad * 2
    bar_w = bar_area_w / n * 0.6
    gap = bar_area_w / n
    bars = []
    text_elems = []
    for i, (lab, v) in enumerate(zip(labels, values, strict=True)):
        x = pad + i * gap + (gap - bar_w) / 2
        h = (height - pad * 2) * (v / max_value)
        y = height - pad - h
        bars.append(
            f'<rect x="{x:.1f}" y="{y:.1f}" width="{bar_w:.1f}" '
            f'height="{h:.1f}" fill="#3b82f6" rx="2" />'
        )
        text_elems.append(
            f'<text x="{x + bar_w / 2:.1f}" y="{height - pad + 14}" '
            f'text-anchor="middle" font-size="11" font-family="{_FONT}" '
            f'fill="#475569">{lab}</text>'
        )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}">'
        f'{"".join(bars)}{"".join(text_elems)}</svg>'
    )


def render_line_chart_svg(
    points: list[tuple[str, float]],
    *,
    width: int = 480,
    height: int = 200,
    max_value: float = 100.0,
) -> str:
    """折线图，points = [(label, value), ...]."""
    if not points:
        raise ValueError("points 不能为空")
    pad = 32
    inner_w = width - pad * 2
    inner_h = height - pad * 2
    n = len(points)
    coords = []
    for i, (_, v) in enumerate(points):
        x = pad + (inner_w * (i / max(1, n - 1)))
        y = pad + inner_h - (inner_h * (v / max_value))
        coords.append((x, y))
    poly = " ".join(f"{x:.1f},{y:.1f}" for x, y in coords)
    text_elems = []
    for (label, _), (x, y) in zip(points, coords, strict=True):
        text_elems.append(
            f'<circle cx="{x:.1f}" cy="{y:.1f}" r="3" fill="#3b82f6" />'
        )
        text_elems.append(
            f'<text x="{x:.1f}" y="{height - 6}" text-anchor="middle" '
            f'font-size="10" font-family="{_FONT}" fill="#475569">{label}</text>'
        )
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}">'
        f'<polyline points="{poly}" fill="none" stroke="#3b82f6" stroke-width="2" />'
        f'{"".join(text_elems)}</svg>'
    )


__all__ = [
    "render_bar_chart_svg",
    "render_line_chart_svg",
    "render_radar_chart_svg",
]
