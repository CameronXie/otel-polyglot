"""Health check models."""

from __future__ import annotations

from pydantic import BaseModel


class HealthResponse(BaseModel):
    model_config = {"extra": "forbid"}

    status: str = "healthy"
