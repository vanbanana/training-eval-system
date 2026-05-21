"""ORM 模型注册."""

from app.models.audit import AuditLog  # noqa: F401
from app.models.chat import ChatMessage, ChatSession  # noqa: F401
from app.models.course import Class, ClassMembership, Course  # noqa: F401
from app.models.evaluation import (  # noqa: F401
    DimensionScore,
    Evaluation,
    EvaluationHistory,
)
from app.models.import_job import ImportJob, ImportRecord  # noqa: F401
from app.models.llm_config import LlmConfig  # noqa: F401
from app.models.notification import Notification, NotificationPref  # noqa: F401
from app.models.profile import StudentProfile  # noqa: F401
from app.models.similarity import SimilarityRecord  # noqa: F401
from app.models.system_config import SystemConfig  # noqa: F401
from app.models.task import Dimension, TrainingTask  # noqa: F401
from app.models.template import EvalTemplate, TemplateDimension  # noqa: F401
from app.models.upload import ParseResult, Upload, VerifyResult  # noqa: F401
from app.models.user import User  # noqa: F401
