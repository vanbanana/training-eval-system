"""业务异常层级 - 全部继承自 BusinessError，由全局处理器统一映射 HTTP 响应."""

from __future__ import annotations

from typing import ClassVar


class BusinessError(Exception):
    """业务异常基类."""

    error_code: ClassVar[str] = "BUSINESS_ERROR"
    http_status: ClassVar[int] = 400

    def __init__(self, message: str, *, field: str | None = None) -> None:
        super().__init__(message)
        self.message = message
        self.field = field

    def __init_subclass__(cls, **kwargs: object) -> None:
        super().__init_subclass__(**kwargs)
        if cls.error_code == "BUSINESS_ERROR" and cls is not BusinessError:
            raise TypeError(f"{cls.__name__} must override class attribute `error_code`")


class ValidationFailedError(BusinessError):
    error_code = "VALIDATION_FAILED"
    http_status = 422


class AuthenticationError(BusinessError):
    error_code = "AUTHENTICATION_FAILED"
    http_status = 401


class AuthorizationError(BusinessError):
    error_code = "AUTHORIZATION_FAILED"
    http_status = 403


class ResourceNotFoundError(BusinessError):
    error_code = "RESOURCE_NOT_FOUND"
    http_status = 404


class ConflictError(BusinessError):
    error_code = "CONFLICT"
    http_status = 409


class BusinessRuleError(BusinessError):
    error_code = "BUSINESS_RULE_VIOLATED"
    http_status = 400


class RateLimitedError(BusinessError):
    error_code = "RATE_LIMITED"
    http_status = 429


class ExternalServiceError(BusinessError):
    error_code = "EXTERNAL_SERVICE_ERROR"
    http_status = 503


class LLMUnavailableError(ExternalServiceError):
    error_code = "LLM_UNAVAILABLE"


class AccountLockedError(AuthenticationError):
    error_code = "ACCOUNT_LOCKED"


class InvalidCredentialsError(AuthenticationError):
    error_code = "INVALID_CREDENTIALS"



class WeightSumInvalidError(BusinessRuleError):
    error_code = "WEIGHT_SUM_INVALID"


class DimensionCountInvalidError(BusinessRuleError):
    error_code = "DIMENSION_COUNT_INVALID"


class DimensionWeightTooLowError(BusinessRuleError):
    error_code = "DIMENSION_WEIGHT_TOO_LOW"


class DimensionsLockedError(BusinessRuleError):
    error_code = "DIMENSIONS_LOCKED"


class FieldLockedError(BusinessRuleError):
    error_code = "FIELD_LOCKED"


class InvalidStatusTransitionError(ConflictError):
    error_code = "INVALID_STATUS_TRANSITION"


class TaskClosedError(ConflictError):
    error_code = "TASK_CLOSED"


class DeadlineInvalidError(BusinessRuleError):
    error_code = "DEADLINE_INVALID"



class UploadTooLargeError(BusinessRuleError):
    error_code = "UPLOAD_TOO_LARGE"


class UploadTooSmallError(BusinessRuleError):
    error_code = "UPLOAD_TOO_SMALL"


class UploadLimitExceededError(BusinessRuleError):
    error_code = "UPLOAD_LIMIT_EXCEEDED"


class FileTypeMismatchError(BusinessRuleError):
    error_code = "FILE_TYPE_MISMATCH"


class TaskClosedForSubmissionError(BusinessRuleError):
    error_code = "TASK_CLOSED_FOR_SUBMISSION"


class NotAssignedError(AuthorizationError):
    error_code = "NOT_ASSIGNED"
