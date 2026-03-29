"""Tests for custom metric definitions."""

from __future__ import annotations

from collections.abc import Generator
from unittest.mock import MagicMock, patch

import pytest

from app.metrics import ForwardMetrics, get_forward_metrics


@pytest.fixture
def mock_counter() -> MagicMock:
    """Mock counter for metrics testing."""
    counter = MagicMock()
    counter.add = MagicMock()
    return counter


@pytest.fixture
def mock_histogram() -> MagicMock:
    """Mock histogram for metrics testing."""
    histogram = MagicMock()
    histogram.record = MagicMock()
    return histogram


@pytest.fixture
def mock_meter(mock_counter: MagicMock, mock_histogram: MagicMock) -> MagicMock:
    """Mock meter for metrics testing."""
    meter = MagicMock()
    meter.create_counter = MagicMock(return_value=mock_counter)
    meter.create_histogram = MagicMock(return_value=mock_histogram)
    return meter


@pytest.fixture(autouse=True)
def mock_metrics(mock_meter: MagicMock) -> Generator:
    """Patch metrics.get_meter and clear cache for each test."""
    with patch("app.metrics.metrics.get_meter", return_value=mock_meter):
        get_forward_metrics.cache_clear()
        yield


@pytest.mark.parametrize(
    "metric_type,name,description,unit",
    [
        ("counter", "forward.requests", "Total outbound forward requests", "1"),
        ("histogram", "forward.duration", "Outbound forward request duration", "s"),
    ],
)
def test_creates_metric(
    metric_type: str, name: str, description: str, unit: str, mock_meter: MagicMock
) -> None:
    """Creates metrics with correct name, description, and unit."""
    ForwardMetrics()

    if metric_type == "counter":
        call = mock_meter.create_counter.call_args
    elif metric_type == "histogram":
        call = mock_meter.create_histogram.call_args
    else:
        raise ValueError(f"Unknown metric type: {metric_type}")

    assert call.kwargs["name"] == name
    assert call.kwargs["description"] == description
    assert call.kwargs["unit"] == unit


def test_record_request(mock_meter: MagicMock) -> None:
    """record_request calls counter.add and histogram.record with correct attributes."""
    mock_counter = MagicMock()
    mock_histogram = MagicMock()
    mock_meter.create_counter.return_value = mock_counter
    mock_meter.create_histogram.return_value = mock_histogram

    metrics = ForwardMetrics()
    metrics.record_request("example.com", "https", 0.5)

    mock_counter.add.assert_called_once_with(
        1,
        attributes={"server.address": "example.com", "url.scheme": "https"},
    )
    mock_histogram.record.assert_called_once_with(
        0.5,
        attributes={"server.address": "example.com", "url.scheme": "https"},
    )


@pytest.mark.parametrize(
    "clear_between,expect_same",
    [
        (False, True),
        (True, False),
    ],
)
def test_get_forward_metrics_cache(clear_between: bool, expect_same: bool) -> None:
    """get_forward_metrics caching behavior works correctly."""
    get_forward_metrics.cache_clear()
    metrics1 = get_forward_metrics()
    if clear_between:
        get_forward_metrics.cache_clear()
    metrics2 = get_forward_metrics()
    assert (metrics1 is metrics2) == expect_same
