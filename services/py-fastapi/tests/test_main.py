"""Tests for FastAPI application factory and entry point."""

from __future__ import annotations

import logging
from unittest.mock import MagicMock, patch

import pytest
from fastapi import FastAPI

from app.config import Settings
from app.main import create_app, lifespan


@pytest.fixture
def mock_init_otel():
    """Mock init_otel as context manager."""
    with patch("app.main.init_otel") as mock:
        mock.return_value.__enter__ = MagicMock(return_value=None)
        mock.return_value.__exit__ = MagicMock(return_value=False)
        yield mock


@pytest.fixture
def mock_span():
    """Mock span with context manager support."""
    span = MagicMock()
    span.__enter__ = MagicMock(return_value=span)
    span.__exit__ = MagicMock(return_value=False)
    return span


@pytest.mark.asyncio
async def test_lifespan(
    test_settings: Settings,
    mock_tracer: MagicMock,
    mock_span: MagicMock,
    mock_init_otel,
    caplog: pytest.LogCaptureFixture,
) -> None:
    """Lifespan creates spans, logs events, and initializes OTel."""
    mock_tracer.start_as_current_span.return_value = mock_span
    app = FastAPI()

    with (
        patch("app.main.get_settings", return_value=test_settings),
        patch("app.main.trace.get_tracer", return_value=mock_tracer),
        caplog.at_level(logging.INFO),
    ):
        async with lifespan(app):
            pass

    # Verify span creation
    mock_tracer.start_as_current_span.assert_any_call("app.startup")

    # Verify OTel initialization
    mock_init_otel.assert_called_once_with(test_settings)

    # Verify logging
    assert any("Starting service" in record.message for record in caplog.records)
    assert any("shutdown" in record.message.lower() for record in caplog.records)


def test_create_app() -> None:
    """create_app returns correctly configured FastAPI instance."""
    with (
        patch("app.main.instrument_fastapi") as mock_instrument,
        patch("app.main.get_settings", return_value=Settings()),
    ):
        app = create_app()

        # Verify type and basic properties
        assert isinstance(app, FastAPI)
        assert app.title == "py-fastapi"
        assert "OpenTelemetry" in app.description

        # Verify lifespan is configured
        assert app.router.lifespan_context is not None

        # Verify routes
        routes = [r.path for r in app.routes]
        assert "/health" in routes
        assert "/forward" in routes

        # Verify instrumentation
        mock_instrument.assert_called_once_with(app)
