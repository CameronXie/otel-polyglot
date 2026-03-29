"""Tests for health check endpoint."""

from __future__ import annotations

from fastapi.testclient import TestClient


def test_health(client: TestClient) -> None:
    """GET /health returns healthy status with correct format."""
    response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "healthy"}
    assert "application/json" in response.headers["content-type"]
