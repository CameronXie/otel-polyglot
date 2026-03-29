"""Tests for OpenTelemetry SDK initialization."""

from __future__ import annotations

import logging
from unittest.mock import MagicMock, patch

import pytest
from opentelemetry.sdk.resources import Resource

from app.config import Settings
from app.otel import (
    create_resource,
    init_logging,
    init_metrics,
    init_otel,
    init_tracing,
)


@pytest.fixture
def settings() -> Settings:
    return Settings(service_name="test-service")


@pytest.fixture
def resource(settings: Settings) -> Resource:
    return create_resource(settings)


@pytest.mark.parametrize(
    "attr_name,expected_value",
    [
        ("service.name", "test-service"),
        ("service.version", None),  # Just check existence
    ],
)
def test_resource_attributes(
    attr_name: str, expected_value: str | None, resource: Resource
) -> None:
    """Resource includes expected attributes."""
    assert attr_name in resource.attributes
    if expected_value is not None:
        assert resource.attributes[attr_name] == expected_value


@pytest.mark.parametrize(
    "log_level,expected",
    [
        (None, logging.INFO),
        ("DEBUG", logging.DEBUG),
        ("WARNING", logging.WARNING),
    ],
)
def test_init_logging(log_level: str | None, expected: int, resource: Resource) -> None:
    """Uses correct log level and attaches handler to root logger."""
    mock_handler = MagicMock()
    root_logger = logging.getLogger()

    with (
        patch("app.otel.set_logger_provider"),
        patch("app.otel.LoggingHandler", return_value=mock_handler) as mock_cls,
    ):
        if log_level:
            init_logging(resource, log_level=log_level)
        else:
            init_logging(resource)

    assert mock_cls.call_args[1]["level"] == expected
    assert mock_handler in root_logger.handlers


def test_init_tracing_uses_resource(resource: Resource) -> None:
    """Passes resource to TracerProvider."""
    with (
        patch("app.otel.OTLPSpanExporter"),
        patch("app.otel.trace.set_tracer_provider"),
    ):
        provider = init_tracing(resource)

    assert provider.resource == resource


def test_init_metrics_uses_resource(resource: Resource) -> None:
    """Passes resource to MeterProvider."""
    with (
        patch("app.otel.OTLPMetricExporter"),
        patch("app.otel.metrics.set_meter_provider"),
    ):
        provider = init_metrics(resource)

    assert provider._sdk_config.resource == resource


def test_init_otel_initializes_all_providers(settings: Settings) -> None:
    """Initializes tracing, metrics, and logging providers."""
    with (
        patch("app.otel.init_tracing") as mock_tracing,
        patch("app.otel.init_metrics") as mock_metrics,
        patch("app.otel.init_logging") as mock_logging,
        patch("app.otel.HTTPXClientInstrumentor"),
    ):
        with init_otel(settings):
            mock_tracing.assert_called_once()
            mock_metrics.assert_called_once()
            mock_logging.assert_called_once()


@pytest.mark.parametrize("raise_exception", [False, True])
def test_init_otel_shutdown(raise_exception: bool, settings: Settings) -> None:
    """Shuts down providers on exit (normal or exception)."""
    mocks = [MagicMock() for _ in range(3)]

    with (
        patch("app.otel.init_tracing", return_value=mocks[0]),
        patch("app.otel.init_metrics", return_value=mocks[1]),
        patch("app.otel.init_logging", return_value=mocks[2]),
        patch("app.otel.HTTPXClientInstrumentor"),
    ):
        if raise_exception:
            with pytest.raises(RuntimeError):
                with init_otel(settings):
                    raise RuntimeError("test")
        else:
            with init_otel(settings):
                pass

    for m in mocks:
        m.shutdown.assert_called_once()


def test_init_otel_instruments_httpx(settings: Settings) -> None:
    """Instruments HTTPX client for automatic spans."""
    mock_instrumentor = MagicMock()

    with (
        patch("app.otel.init_tracing", return_value=MagicMock()),
        patch("app.otel.init_metrics", return_value=MagicMock()),
        patch("app.otel.init_logging", return_value=MagicMock()),
        patch("app.otel.HTTPXClientInstrumentor") as mock_class,
    ):
        mock_class.return_value = mock_instrumentor

        with init_otel(settings):
            pass

    mock_instrumentor.instrument.assert_called_once()
