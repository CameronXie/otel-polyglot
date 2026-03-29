"""Tests for application configuration."""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.config import Settings, get_settings


@pytest.fixture(autouse=True)
def clear_settings_cache() -> None:
    """Clear settings cache before each test."""
    get_settings.cache_clear()


@pytest.mark.parametrize(
    "input_dict,expected_values,should_be_valid",
    [
        # Valid cases - only check fields we care about
        ({}, {}, True),  # all defaults
        ({"port": 9000}, {"port": 9000}, True),
        ({"port": 1}, {"port": 1}, True),
        ({"port": 65535}, {"port": 65535}, True),
        ({"log_level": "DEBUG"}, {"log_level": "DEBUG"}, True),
        ({"log_level": "debug"}, {"log_level": "DEBUG"}, True),
        ({"log_level": "Info"}, {"log_level": "INFO"}, True),
        ({"log_level": "WARNING"}, {"log_level": "WARNING"}, True),
        ({"log_level": "ERROR"}, {"log_level": "ERROR"}, True),
        ({"log_level": "CRITICAL"}, {"log_level": "CRITICAL"}, True),
        ({"forward_urls": ""}, {"forward_urls": []}, True),
        (
            {"forward_urls": "https://a.com,https://b.com"},
            {"forward_urls": ["https://a.com", "https://b.com"]},
            True,
        ),
        (
            {"forward_urls": ["https://a.com"]},
            {"forward_urls": ["https://a.com"]},
            True,
        ),
        (
            {"forward_urls": "  https://a.com ,  https://b.com  "},
            {"forward_urls": ["https://a.com", "https://b.com"]},
            True,
        ),
        (
            {"forward_urls": "https://a.com,,https://b.com,"},
            {"forward_urls": ["https://a.com", "https://b.com"]},
            True,
        ),
        (
            {
                "forward_urls": [
                    "http://example.com",
                    "https://example.com/api/v1",
                    "https://example.com:8443",
                ]
            },
            {
                "forward_urls": [
                    "http://example.com",
                    "https://example.com/api/v1",
                    "https://example.com:8443",
                ]
            },
            True,
        ),
        # Invalid cases
        ({"port": 0}, None, False),
        ({"port": -1}, None, False),
        ({"port": 65536}, None, False),
        ({"log_level": "INVALID"}, None, False),
        ({"log_level": "trace"}, None, False),
        ({"log_level": ""}, None, False),
        ({"forward_urls": ["example.com"]}, None, False),  # no scheme
        ({"forward_urls": ["https://"]}, None, False),  # no netloc
    ],
)
def test_settings_validation(
    input_dict: dict[str, object],
    expected_values: dict[str, object],
    should_be_valid: bool,
) -> None:
    """Settings validation works correctly for all field types."""
    if should_be_valid:
        settings = Settings(**input_dict)
        for key, value in expected_values.items():
            assert getattr(settings, key) == value
    else:
        with pytest.raises(ValidationError):
            Settings(**input_dict)


@pytest.mark.parametrize(
    "env_vars,expected_values,expected_service_name",
    [
        # Individual env vars
        ({"PY_FASTAPI_LOG_LEVEL": "DEBUG"}, {"log_level": "DEBUG"}, "unknown_service"),
        ({"PY_FASTAPI_PORT": "9000"}, {"port": 9000}, "unknown_service"),
        (
            {"PY_FASTAPI_SERVICE_NAME": "my-service"},
            {"service_name": "my-service"},
            "my-service",
        ),
        (
            {"PY_FASTAPI_FORWARD_URLS": "https://a.com,https://b.com"},
            {"forward_urls": ["https://a.com", "https://b.com"]},
            "unknown_service",
        ),
        # Service name fallback behavior
        ({}, {}, "unknown_service"),
        ({"OTEL_SERVICE_NAME": "otel-service"}, {}, "otel-service"),
        (
            {
                "PY_FASTAPI_SERVICE_NAME": "my-service",
                "OTEL_SERVICE_NAME": "otel-service",
            },
            {"service_name": "my-service"},
            "my-service",
        ),
    ],
)
def test_env_loading(
    monkeypatch: pytest.MonkeyPatch,
    env_vars: dict[str, str],
    expected_values: dict[str, object],
    expected_service_name: str,
) -> None:
    """Settings load correctly from environment variables."""
    for key, value in env_vars.items():
        monkeypatch.setenv(key, value)
    settings = get_settings()
    for key, value in expected_values.items():
        assert getattr(settings, key) == value
    assert settings.get_service_name() == expected_service_name


@pytest.mark.parametrize(
    "clear_between,expect_same",
    [
        (False, True),  # no clear → same instance
        (True, False),  # clear → different instance
    ],
)
def test_settings_cache(clear_between: bool, expect_same: bool) -> None:
    """get_settings caching behavior works correctly."""
    get_settings.cache_clear()
    settings1 = get_settings()
    if clear_between:
        get_settings.cache_clear()
    settings2 = get_settings()
    assert (settings1 is settings2) == expect_same


def test_extra_env_vars_ignored(monkeypatch: pytest.MonkeyPatch) -> None:
    """Extra environment variables are ignored (Pydantic-settings behavior)."""
    monkeypatch.setenv("PY_FASTAPI_UNKNOWN_FIELD", "value")
    settings = get_settings()  # Should not raise, just ignore the unknown field
    assert isinstance(settings, Settings)
