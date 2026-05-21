"""Domain 评分纯函数（Epic 16.2）.

Property 1: total = Σ(weight_i × (obj_i × α + subj_i × (1-α))) / 100
Property 2: total ∈ [0, 100]
Property 3: weights 总和必须 = 100
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import ROUND_HALF_UP, Decimal


@dataclass(frozen=True)
class DimensionScoreData:
    """评分计算输入项."""

    weight: int  # 1-100
    objective_score: float | None  # 0-100
    subjective_score: float | None  # 0-100


def _validate_score(value: float | None) -> None:
    if value is None:
        return
    if value < 0 or value > 100:
        raise ValueError(f"score {value} out of range [0, 100]")


def compute_final_score(
    scores: list[DimensionScoreData], alpha: float
) -> Decimal:
    """计算综合得分.

    - alpha ∈ [0, 1]：客观分权重
    - subj 缺失 → fallback obj
    - obj 缺失 → fallback subj
    - 都缺失 → 视为 0（仍计入 weight）
    - 结果保留 1 位小数
    """
    if not 0 <= alpha <= 1:
        raise ValueError(f"alpha {alpha} 必须在 [0, 1]")
    if not scores:
        raise ValueError("scores 不能为空")
    total_weight = sum(s.weight for s in scores)
    if total_weight != 100:
        raise ValueError(
            f"weights sum {total_weight} != 100"
        )

    total = Decimal("0")
    for s in scores:
        if s.weight < 0 or s.weight > 100:
            raise ValueError(f"weight {s.weight} out of [0, 100]")
        _validate_score(s.objective_score)
        _validate_score(s.subjective_score)

        obj = s.objective_score
        subj = s.subjective_score
        if obj is None and subj is None:
            point = Decimal("0")
        elif obj is None:
            point = Decimal(str(subj))
        elif subj is None:
            point = Decimal(str(obj))
        else:
            point = (
                Decimal(str(obj)) * Decimal(str(alpha))
                + Decimal(str(subj)) * (Decimal("1") - Decimal(str(alpha)))
            )
        total += point * Decimal(s.weight) / Decimal("100")

    rounded = total.quantize(Decimal("0.1"), rounding=ROUND_HALF_UP)
    if rounded < 0:
        return Decimal("0.0")
    if rounded > Decimal("100"):
        return Decimal("100.0")
    return rounded
