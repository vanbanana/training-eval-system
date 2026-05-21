"""Profile Skills - Epic 18."""

from app.llm.skills.profile.weakness_analyze import (
    Weakness,
    WeaknessAnalyzeSkill,
    WeaknessInput,
    WeaknessOutput,
)
from app.llm.skills.profile.learning_advice import (
    AdviceInput,
    AdviceOutput,
    LearningAdviceSkill,
    Suggestion,
)
from app.llm.skills.profile.teaching_summary import (
    CommonWeakness,
    TeachingSummaryInput,
    TeachingSummaryOutput,
    TeachingSummarySkill,
)


__all__ = [
    "AdviceInput",
    "AdviceOutput",
    "CommonWeakness",
    "LearningAdviceSkill",
    "Suggestion",
    "TeachingSummaryInput",
    "TeachingSummaryOutput",
    "TeachingSummarySkill",
    "Weakness",
    "WeaknessAnalyzeSkill",
    "WeaknessInput",
    "WeaknessOutput",
]
