"""Pytest configuration and shared fixtures."""

from __future__ import annotations

import os
from collections.abc import Generator
from unittest.mock import MagicMock

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.config import Settings, get_settings
from app.metrics import get_forward_metrics
from app.routers import forward_router, healthcheck_router


@pytest.fixture(autouse=True)
def clear_caches() -> Generator[None]:
    """Clear LRU caches before and after each test."""
    get_settings.cache_clear()
    get_forward_metrics.cache_clear()
    # Clear any environment variables that might affect settings
    for key in list(os.environ.keys()):
        if key.startswith("PY_FASTAPI_") or key == "OTEL_SERVICE_NAME":
            del os.environ[key]
    yield
    get_settings.cache_clear()
    get_forward_metrics.cache_clear()


@pytest.fixture
def test_settings() -> Settings:
    """Isolated Settings for testing."""
    return Settings(
        port=8080,
        forward_urls=[],
        service_name="test-service",
    )


@pytest.fixture
def app(test_settings: Settings) -> FastAPI:
    """Test FastAPI app without OTel instrumentation."""
    test_app = FastAPI(
        title="test-py-fastapi",
        description="Test FastAPI service",
        version="0.0.0-test",
    )
    test_app.include_router(healthcheck_router)
    test_app.include_router(forward_router)
    test_app.dependency_overrides[get_settings] = lambda: test_settings
    return test_app


@pytest.fixture
def client(app: FastAPI) -> TestClient:
    """Synchronous test client."""
    return TestClient(app)


@pytest.fixture
def mock_tracer() -> MagicMock:
    """Mock tracer with context manager span."""
    tracer = MagicMock()
    span = MagicMock()
    span.__enter__ = MagicMock(return_value=span)
    span.__exit__ = MagicMock(return_value=False)
    tracer.start_as_current_span.return_value = span
    return tracer
