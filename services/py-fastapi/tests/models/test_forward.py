"""Tests for forward endpoint models."""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from app.models.forward import ErrorResponse, ForwardResponse, ForwardResult


@pytest.mark.parametrize(
    "kwargs,expected_dump",
    [
        (
            {"url": "https://example.com", "duration_seconds": 0.5},
            {
                "url": "https://example.com",
                "duration_seconds": 0.5,
                "status_code": None,
                "body": None,
                "error": None,
            },
        ),
        (
            {"url": "https://test.com", "duration_seconds": 1.0, "status_code": 200},
            {
                "url": "https://test.com",
                "duration_seconds": 1.0,
                "status_code": 200,
                "body": None,
                "error": None,
            },
        ),
        (
            {
                "url": "https://test.com",
                "duration_seconds": 0.1,
                "body": "response body",
            },
            {
                "url": "https://test.com",
                "duration_seconds": 0.1,
                "status_code": None,
                "body": "response body",
                "error": None,
            },
        ),
        (
            {"url": "https://test.com", "duration_seconds": 0.2, "error": "failed"},
            {
                "url": "https://test.com",
                "duration_seconds": 0.2,
                "status_code": None,
                "body": None,
                "error": "failed",
            },
        ),
        (
            {
                "url": "https://test.com",
                "duration_seconds": 0.3,
                "status_code": 404,
                "body": "not found",
                "error": None,
            },
            {
                "url": "https://test.com",
                "duration_seconds": 0.3,
                "status_code": 404,
                "body": "not found",
                "error": None,
            },
        ),
    ],
)
def test_forward_result(kwargs: dict, expected_dump: dict) -> None:
    """ForwardResult fields are set correctly with expected defaults."""
    result = ForwardResult(**kwargs)
    assert result.model_dump() == expected_dump


def test_forward_response_defaults() -> None:
    """results defaults to empty list."""
    assert ForwardResponse().results == []


def test_error_response_requires_error() -> None:
    """error field is required."""
    with pytest.raises(ValidationError):
        ErrorResponse()  # type: ignore
