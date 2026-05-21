"""Task 1.1 验收：Settings 配置类."""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.core.config import Settings, get_settings


class TestSettingsHappy:
    """主路径：合法环境变量加载."""

    def test_load_with_valid_env(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given 完整合法环境变量；When 实例化 Settings；Then 字段正确读取且默认值生效."""
        monkeypatch.setenv("TES_ENV", "dev")
        monkeypatch.setenv(
            "TES_DB_URL", "postgresql+asyncpg://u:p@h:5432/d"
        )
        monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
        monkeypatch.setenv("TES_LLM_KEY_MASTER", "y" * 44)

        get_settings.cache_clear()
        settings = Settings()

        assert settings.env == "dev"
        assert "postgresql+asyncpg" in settings.db_url
        assert settings.max_upload_size_mb == 50  # 默认值

    def test_get_settings_is_singleton(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given 多次调用 get_settings；When 比较；Then 同一实例（lru_cache）."""
        monkeypatch.setenv("TES_JWT_SECRET", "z" * 32)
        get_settings.cache_clear()
        a = get_settings()
        b = get_settings()
        assert a is b


class TestSettingsValidation:
    """异常路径：非法配置触发 ValidationError."""

    def test_reject_short_jwt_secret(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given JWT_SECRET 长度 < 32；When 实例化；Then 抛 ValidationError 含长度提示."""
        monkeypatch.setenv("TES_JWT_SECRET", "short")

        with pytest.raises(ValidationError) as exc_info:
            Settings()
        assert "32" in str(exc_info.value)

    def test_reject_invalid_upload_size_zero(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given MAX_UPLOAD_SIZE_MB = 0；When 实例化；Then ValidationError."""
        monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
        monkeypatch.setenv("TES_MAX_UPLOAD_SIZE_MB", "0")
        with pytest.raises(ValidationError) as exc_info:
            Settings()
        # pydantic v2 错误消息：greater than or equal to 1
        assert "greater than or equal to 1" in str(exc_info.value).lower() or "1" in str(exc_info.value)

    def test_reject_invalid_upload_size_too_large(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given MAX_UPLOAD_SIZE_MB = 501；When 实例化；Then ValidationError."""
        monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
        monkeypatch.setenv("TES_MAX_UPLOAD_SIZE_MB", "501")
        with pytest.raises(ValidationError):
            Settings()


class TestSettingsBoundary:
    """边界路径：env literal、空字段处理."""

    def test_env_literal_rejects_invalid(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given env=staging（非 dev/test/prod）；When 实例化；Then ValidationError."""
        monkeypatch.setenv("TES_ENV", "staging")
        monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
        with pytest.raises(ValidationError):
            Settings()

    def test_default_values_filled_when_env_unset(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Given 仅设置必需字段；When 实例化；Then 默认值生效."""
        monkeypatch.setenv("TES_JWT_SECRET", "x" * 32)
        monkeypatch.delenv("TES_MAX_UPLOAD_SIZE_MB", raising=False)
        get_settings.cache_clear()
        s = Settings()
        assert s.max_upload_size_mb == 50
        assert s.upload_root == "./data/uploads"
        assert s.jwt_algorithm == "HS256"
