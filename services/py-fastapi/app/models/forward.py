"""Forward endpoint request and response models."""

from __future__ import annotations

from pydantic import BaseModel, Field


class ForwardResult(BaseModel):
    model_config = {"extra": "forbid"}

    url: str = Field(..., description="Requested URL")
    status_code: int | None = Field(default=None, description="HTTP status code")
    body: str | None = Field(default=None, description="Response body")
    error: str | None = Field(default=None, description="Error if failed")
    duration_seconds: float | None = Field(
        default=None, description="Duration in seconds"
    )


class ForwardResponse(BaseModel):
    model_config = {"extra": "forbid"}

    results: list[ForwardResult] = Field(
        default_factory=list, description="Forwarded request results"
    )


class ErrorResponse(BaseModel):
    model_config = {"extra": "forbid"}

    error: str = Field(..., description="Error message")
