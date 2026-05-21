"""Epic 16.2 验收：评分纯函数 - Property 1/2/3 重点验证."""

from __future__ import annotations

from decimal import Decimal

import pytest

from app.services.scoring import DimensionScoreData, compute_final_score


pytestmark = pytest.mark.unit


class TestComputeFinalScoreHappyPath:
    def test_single_dim_obj80_subj90_alpha06(self) -> None:
        """obj=80 subj=90 alpha=0.6 weight=100 → 80*0.6 + 90*0.4 = 84.0."""
        scores = [
            DimensionScoreData(weight=100, objective_score=80, subjective_score=90)
        ]
        assert compute_final_score(scores, alpha=0.6) == Decimal("84.0")

    def test_two_dims_weighted(self) -> None:
        """50%权重各，obj=70 subj=80 obj=90 subj=100 alpha=0.5。"""
        scores = [
            DimensionScoreData(weight=50, objective_score=70, subjective_score=80),
            DimensionScoreData(weight=50, objective_score=90, subjective_score=100),
        ]
        # dim1: 75; dim2: 95; total = 75*0.5 + 95*0.5 = 85
        assert compute_final_score(scores, alpha=0.5) == Decimal("85.0")

    def test_alpha_zero_only_subjective(self) -> None:
        """alpha=0 → 仅看主观分."""
        scores = [
            DimensionScoreData(weight=100, objective_score=20, subjective_score=80)
        ]
        assert compute_final_score(scores, alpha=0.0) == Decimal("80.0")

    def test_alpha_one_only_objective(self) -> None:
        scores = [
            DimensionScoreData(weight=100, objective_score=20, subjective_score=80)
        ]
        assert compute_final_score(scores, alpha=1.0) == Decimal("20.0")


class TestFallback:
    def test_subjective_missing_uses_objective(self) -> None:
        scores = [
            DimensionScoreData(weight=100, objective_score=70, subjective_score=None)
        ]
        assert compute_final_score(scores, alpha=0.6) == Decimal("70.0")

    def test_objective_missing_uses_subjective(self) -> None:
        scores = [
            DimensionScoreData(weight=100, objective_score=None, subjective_score=70)
        ]
        assert compute_final_score(scores, alpha=0.6) == Decimal("70.0")

    def test_both_missing_zero(self) -> None:
        scores = [
            DimensionScoreData(weight=100, objective_score=None, subjective_score=None)
        ]
        assert compute_final_score(scores, alpha=0.6) == Decimal("0.0")


class TestBoundary:
    def test_zero_score(self) -> None:
        scores = [DimensionScoreData(weight=100, objective_score=0, subjective_score=0)]
        assert compute_final_score(scores, alpha=0.5) == Decimal("0.0")

    def test_full_score(self) -> None:
        scores = [
            DimensionScoreData(weight=100, objective_score=100, subjective_score=100)
        ]
        assert compute_final_score(scores, alpha=0.5) == Decimal("100.0")

    def test_weight_sum_not_100_raises(self) -> None:
        scores = [DimensionScoreData(weight=50, objective_score=80, subjective_score=80)]
        with pytest.raises(ValueError, match="!= 100"):
            compute_final_score(scores, alpha=0.5)

    def test_score_out_of_range_raises(self) -> None:
        scores = [
            DimensionScoreData(weight=100, objective_score=101, subjective_score=80)
        ]
        with pytest.raises(ValueError, match="out of range"):
            compute_final_score(scores, alpha=0.5)

    def test_alpha_out_of_range_raises(self) -> None:
        scores = [DimensionScoreData(weight=100, objective_score=80, subjective_score=80)]
        with pytest.raises(ValueError, match="alpha"):
            compute_final_score(scores, alpha=1.5)

    def test_empty_scores_raises(self) -> None:
        with pytest.raises(ValueError, match="不能为空"):
            compute_final_score([], alpha=0.5)
