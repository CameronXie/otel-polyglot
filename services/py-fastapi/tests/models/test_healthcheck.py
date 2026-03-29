"""Tests for health check models."""

from __future__ import annotations

import pytest

from app.models.healthcheck import HealthResponse


@pytest.mark.parametrize(
    "kwargs,expected_status",
    [
        ({}, "healthy"),
        ({"status": "degraded"}, "degraded"),
        ({"status": "unhealthy"}, "unhealthy"),
    ],
)
def test_health_response(kwargs: dict, expected_status: str) -> None:
    """HealthResponse status field works correctly with default and custom values."""
    response = HealthResponse(**kwargs)
    assert response.status == expected_status
