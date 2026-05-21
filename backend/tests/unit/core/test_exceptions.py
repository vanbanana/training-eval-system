"""Task 1.2 验收：业务异常类层级."""

from __future__ import annotations

import pytest

from app.core.exceptions import (
    AccountLockedError,
    AuthenticationError,
    AuthorizationError,
    BusinessError,
    BusinessRuleError,
    ConflictError,
    ExternalServiceError,
    InvalidCredentialsError,
    LLMUnavailableError,
    RateLimitedError,
    ResourceNotFoundError,
    ValidationFailedError,
)


class TestBusinessErrorAttributes:
    """主路径：实例化业务异常并读取标准属性."""

    def test_business_rule_error_attributes(self) -> None:
        """Given BusinessRuleError(message='sum=85', field='dimensions');
        When 读取属性；Then error_code='BUSINESS_RULE_VIOLATED'/http=400/field='dimensions'."""
        err = BusinessRuleError("sum=85", field="dimensions")
        assert err.error_code == "BUSINESS_RULE_VIOLATED"
        assert err.http_status == 400
        assert err.field == "dimensions"
        assert str(err) == "sum=85"

    def test_validation_failed_maps_to_422(self) -> None:
        err = ValidationFailedError("invalid")
        assert err.error_code == "VALIDATION_FAILED"
        assert err.http_status == 422

    def test_authentication_error_maps_to_401(self) -> None:
        assert AuthenticationError("x").http_status == 401

    def test_authorization_error_maps_to_403(self) -> None:
        assert AuthorizationError("x").http_status == 403

    def test_resource_not_found_maps_to_404(self) -> None:
        assert ResourceNotFoundError("x").http_status == 404

    def test_conflict_maps_to_409(self) -> None:
        assert ConflictError("x").http_status == 409

    def test_rate_limited_maps_to_429(self) -> None:
        assert RateLimitedError("x").http_status == 429

    def test_external_service_maps_to_503(self) -> None:
        assert ExternalServiceError("x").http_status == 503


class TestSubclassValidation:
    """异常路径：子类必须覆盖 error_code."""

    def test_subclass_must_define_error_code(self) -> None:
        """Given 子类未覆盖 error_code；When 类定义；Then 抛 TypeError."""
        with pytest.raises(TypeError) as exc_info:
            class _Foo(BusinessError):  # 没有覆盖 error_code
                pass

        assert "error_code" in str(exc_info.value)


class TestInheritanceChain:
    """边界路径：继承层级 isinstance 校验."""

    def test_llm_unavailable_inherits_external(self) -> None:
        """Given LLMUnavailableError；When isinstance 检查；
        Then 既是 ExternalServiceError 又是 BusinessError."""
        e = LLMUnavailableError("timeout")
        assert isinstance(e, ExternalServiceError)
        assert isinstance(e, BusinessError)
        assert e.error_code == "LLM_UNAVAILABLE"
        assert e.http_status == 503

    def test_account_locked_inherits_authentication(self) -> None:
        e = AccountLockedError("locked until 12:00")
        assert isinstance(e, AuthenticationError)
        assert e.error_code == "ACCOUNT_LOCKED"
        assert e.http_status == 401

    def test_invalid_credentials_inherits_authentication(self) -> None:
        e = InvalidCredentialsError("bad pw")
        assert isinstance(e, AuthenticationError)
        assert e.error_code == "INVALID_CREDENTIALS"


class TestErrorCodeUniqueness:
    """边界路径：error_code 全大写下划线、不重复."""

    def test_all_error_codes_uppercase_and_unique(self) -> None:
        all_classes = [
            ValidationFailedError, AuthenticationError, AuthorizationError,
            ResourceNotFoundError, ConflictError, BusinessRuleError,
            RateLimitedError, ExternalServiceError, LLMUnavailableError,
            AccountLockedError, InvalidCredentialsError,
        ]
        codes = [c.error_code for c in all_classes]
        # 大小写校验
        for code in codes:
            assert code == code.upper(), f"{code} 必须全大写"
            assert " " not in code
        # 唯一性
        assert len(set(codes)) == len(codes), "error_code 重复"
