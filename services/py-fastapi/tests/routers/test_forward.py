"""Comprehensive tests for forward endpoint with OpenTelemetry instrumentation."""

from __future__ import annotations

import asyncio
import logging
from collections.abc import AsyncGenerator
from contextlib import ExitStack
from dataclasses import dataclass
from unittest.mock import MagicMock, patch

import httpx
import pytest
import respx
from fastapi import FastAPI
from httpx import ASGITransport, AsyncClient
from opentelemetry import trace
from opentelemetry.semconv.attributes.http_attributes import (
    HTTP_REQUEST_METHOD,
    HTTP_RESPONSE_STATUS_CODE,
)

from app.config import Settings
from app.metrics import get_forward_metrics
from app.routers.forward import (
    _forward_batch,
    _forward_single,
    _read_limited_body,
)


@pytest.fixture
async def async_client(app: FastAPI) -> AsyncGenerator[AsyncClient]:
    """Async test client."""
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://test",
    ) as ac:
        yield ac


@pytest.fixture
def mock_tracer() -> MagicMock:
    """Mock tracer with context manager span."""
    tracer = MagicMock()
    span = MagicMock()
    span.__enter__ = MagicMock(return_value=span)
    span.__exit__ = MagicMock(return_value=False)
    tracer.start_as_current_span.return_value = span
    return tracer


@pytest.mark.parametrize(
    "_name,chunks,limit,encoding,expected,expected_contains",
    [
        pytest.param("empty", [], 1024, "utf-8", "", None, id="empty"),
        pytest.param(
            "single-chunk", [b"Hello"], 1024, "utf-8", "Hello", None, id="single-chunk"
        ),
        pytest.param(
            "exact-limit",
            [b"x" * 100],
            100,
            "utf-8",
            "x" * 100,
            None,
            id="exact-limit",
        ),
        pytest.param(
            "exceeds-limit",
            [b"x" * 200],
            100,
            "utf-8",
            "x" * 100,
            None,
            id="exceeds-limit",
        ),
        pytest.param(
            "multiple-under",
            [b"a", b"b", b"c"],
            1024,
            "utf-8",
            "abc",
            None,
            id="multiple-under",
        ),
        pytest.param(
            "multiple-exceeds",
            [b"a", b"b", b"c"],
            2,
            "utf-8",
            "ab",
            None,
            id="multiple-exceeds",
        ),
        pytest.param(
            "unicode", ["世界".encode()], 1024, "utf-8", "世界", None, id="unicode"
        ),
        pytest.param("latin1", [b"\xe9"], 1024, "latin-1", "é", None, id="latin1"),
        pytest.param(
            "missing-encoding",
            [b"Hello"],
            1024,
            None,
            "Hello",
            None,
            id="missing-encoding",
        ),
        pytest.param(
            "invalid-utf8",
            [b"\xff\xfe"],
            1024,
            "utf-8",
            None,
            "\ufffd",
            id="invalid-utf8",
        ),
    ],
)
@pytest.mark.asyncio
async def test_read_limited_body(
    _name: str,
    chunks: list[bytes],
    limit: int,
    encoding: str | None,
    expected: str | None,
    expected_contains: str | None,
) -> None:
    """_read_limited_body handles various inputs correctly."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.encoding = encoding

    async def aiter_bytes_impl():
        for chunk in chunks:
            yield chunk

    mock_response.aiter_bytes = aiter_bytes_impl

    result = await _read_limited_body(mock_response, limit)

    if expected is not None:
        assert result == expected
    if expected_contains is not None:
        assert expected_contains in result


@pytest.mark.parametrize(
    "_name,urls,mock_responses,expected_count,expected_status_codes",
    [
        pytest.param("empty", [], {}, 0, [], id="empty"),
        pytest.param(
            "single",
            ["https://example.com/api"],
            {"https://example.com/api": (200, {"ok": True})},
            1,
            [200],
            id="single",
        ),
        pytest.param(
            "multiple",
            ["https://a.com/api", "https://b.com/api"],
            {
                "https://a.com/api": (200, {}),
                "https://b.com/api": (201, {}),
            },
            2,
            [200, 201],
            id="multiple",
        ),
        pytest.param(
            "mixed-success-failure",
            ["https://a.com/api", "https://b.com/api"],
            {
                "https://a.com/api": (200, {"data": "ok"}),
                "https://b.com/api": (500, {"error": "fail"}),
            },
            2,
            [200, 500],
            id="mixed-success-failure",
        ),
    ],
)
@pytest.mark.asyncio
async def test_forward_batch(
    _name: str,
    urls: list[str],
    mock_responses: dict[str, tuple[int, dict]],
    expected_count: int,
    expected_status_codes: list[int],
    mock_tracer: MagicMock,
) -> None:
    """_forward_batch handles various URL configurations correctly."""
    with respx.mock:
        for url, (status, json_body) in mock_responses.items():
            respx.get(url).mock(return_value=httpx.Response(status, json=json_body))

        results = await _forward_batch(urls, mock_tracer)

    assert len(results) == expected_count
    for i, status_code in enumerate(expected_status_codes):
        assert results[i].status_code == status_code
        assert results[i].error is None


@pytest.mark.asyncio
async def test_forward_batch_parallel_execution(mock_tracer: MagicMock) -> None:
    """_forward_batch executes requests in parallel."""
    delay = 0.1

    async def slow_response(_: httpx.Request) -> httpx.Response:
        await asyncio.sleep(delay)
        return httpx.Response(200, json={})

    with respx.mock:
        respx.get("https://a.com/api").mock(side_effect=slow_response)
        respx.get("https://b.com/api").mock(side_effect=slow_response)

        import time

        start = time.perf_counter()
        results = await _forward_batch(
            ["https://a.com/api", "https://b.com/api"], mock_tracer
        )
        elapsed = time.perf_counter() - start

    assert len(results) == 2
    assert elapsed < 2 * delay


@dataclass
class ForwardSingleTestCase:
    """Test case definition for _forward_single comprehensive tests."""

    name: str
    status_code: int | None
    exception: tuple[type[Exception], str] | None
    expected_status_code: int | None
    expected_error_contains: str | None
    expected_span_status: trace.StatusCode
    expect_exception_recorded: bool
    expect_metrics_recorded: bool


FORWARD_SINGLE_CASES = [
    # Happy path - 2xx
    ForwardSingleTestCase(
        name="200-ok",
        status_code=200,
        exception=None,
        expected_status_code=200,
        expected_error_contains=None,
        expected_span_status=trace.StatusCode.OK,
        expect_exception_recorded=False,
        expect_metrics_recorded=True,
    ),
    ForwardSingleTestCase(
        name="201-ok",
        status_code=201,
        exception=None,
        expected_status_code=201,
        expected_error_contains=None,
        expected_span_status=trace.StatusCode.OK,
        expect_exception_recorded=False,
        expect_metrics_recorded=True,
    ),
    # Happy path - 3xx
    ForwardSingleTestCase(
        name="302-ok",
        status_code=302,
        exception=None,
        expected_status_code=302,
        expected_error_contains=None,
        expected_span_status=trace.StatusCode.OK,
        expect_exception_recorded=False,
        expect_metrics_recorded=True,
    ),
    # Sad path - 4xx
    ForwardSingleTestCase(
        name="404-error",
        status_code=404,
        exception=None,
        expected_status_code=404,
        expected_error_contains=None,
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=False,
        expect_metrics_recorded=True,
    ),
    # Sad path - 5xx
    ForwardSingleTestCase(
        name="500-error",
        status_code=500,
        exception=None,
        expected_status_code=500,
        expected_error_contains=None,
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=False,
        expect_metrics_recorded=True,
    ),
    # HTTP errors - no metrics recorded
    ForwardSingleTestCase(
        name="connect-error",
        status_code=None,
        exception=(httpx.ConnectError, "Connection failed"),
        expected_status_code=None,
        expected_error_contains="Connection failed",
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=True,
        expect_metrics_recorded=False,
    ),
    ForwardSingleTestCase(
        name="timeout-error",
        status_code=None,
        exception=(httpx.TimeoutException, "Request timed out"),
        expected_status_code=None,
        expected_error_contains="Request timed out",
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=True,
        expect_metrics_recorded=False,
    ),
    ForwardSingleTestCase(
        name="read-error",
        status_code=None,
        exception=(httpx.ReadError, "Read error"),
        expected_status_code=None,
        expected_error_contains="Read error",
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=True,
        expect_metrics_recorded=False,
    ),
    ForwardSingleTestCase(
        name="write-error",
        status_code=None,
        exception=(httpx.WriteError, "Write error"),
        expected_status_code=None,
        expected_error_contains="Write error",
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=True,
        expect_metrics_recorded=False,
    ),
    # Generic exception
    ForwardSingleTestCase(
        name="runtime-error",
        status_code=None,
        exception=(RuntimeError, "Unexpected"),
        expected_status_code=None,
        expected_error_contains="Unexpected",
        expected_span_status=trace.StatusCode.ERROR,
        expect_exception_recorded=True,
        expect_metrics_recorded=False,
    ),
]


@pytest.mark.parametrize("case", FORWARD_SINGLE_CASES, ids=lambda c: c.name)
@pytest.mark.asyncio
async def test_forward_single(
    case: ForwardSingleTestCase, mock_tracer: MagicMock
) -> None:
    """_forward_single handles all scenarios comprehensively."""
    mock_counter = MagicMock()
    mock_histogram = MagicMock()

    with (
        respx.mock,
        patch("app.metrics.metrics.get_meter") as mock_get_meter,
    ):
        mock_meter = MagicMock()
        mock_meter.create_counter.return_value = mock_counter
        mock_meter.create_histogram.return_value = mock_histogram
        mock_get_meter.return_value = mock_meter

        get_forward_metrics.cache_clear()
        metrics_instance = get_forward_metrics()

        if case.exception is not None:
            exc_type, exc_msg = case.exception
            respx.get("https://example.com/api").mock(side_effect=exc_type(exc_msg))
        else:
            respx.get("https://example.com/api").mock(
                return_value=httpx.Response(case.status_code, json={"data": "test"})
            )

        async with httpx.AsyncClient() as client:
            result = await _forward_single(
                "https://example.com/api", client, mock_tracer, metrics_instance
            )

    assert result.status_code == case.expected_status_code
    assert result.duration_seconds is not None
    assert result.duration_seconds >= 0

    if case.expected_error_contains is not None:
        assert result.error is not None
        assert case.expected_error_contains in result.error
    else:
        assert result.error is None

    span = mock_tracer.start_as_current_span.return_value
    status_call = span.set_status.call_args
    assert status_call is not None
    status_arg = status_call[0][0]
    assert status_arg.status_code == case.expected_span_status

    if case.status_code is not None:
        assert result.body is not None

        span.set_attributes.assert_called_once()
        attrs = span.set_attributes.call_args[0][0]
        assert attrs[HTTP_RESPONSE_STATUS_CODE] == case.status_code
        assert attrs[HTTP_REQUEST_METHOD] == "GET"
    else:
        span.set_attributes.assert_not_called()

    if case.expect_exception_recorded:
        span.record_exception.assert_called()
    else:
        span.record_exception.assert_not_called()

    if case.expect_metrics_recorded:
        mock_counter.add.assert_called_once()
        mock_histogram.record.assert_called_once()
    else:
        mock_counter.add.assert_not_called()
        mock_histogram.record.assert_not_called()


@dataclass
class ForwardRequestsTestCase:
    """Test case definition for forward_requests endpoint comprehensive tests."""

    name: str
    requests: list[tuple[str, int, dict]]  # (url, status_code, response_body)
    baggage: dict[str, str] | None
    batch_exception: type[Exception] | None
    expected_status: int
    expected_results_count: int | None
    expected_error_detail: str | None
    expected_log_message: str | None
    expected_log_level: int | None


FORWARD_REQUESTS_CASES = [
    ForwardRequestsTestCase(
        name="empty-url-list",
        requests=[],
        baggage=None,
        batch_exception=None,
        expected_status=200,
        expected_results_count=0,
        expected_error_detail=None,
        expected_log_message="Starting forward batch",
        expected_log_level=logging.INFO,
    ),
    ForwardRequestsTestCase(
        name="single-url-success",
        requests=[("https://example.com/api", 200, {"data": "test"})],
        baggage=None,
        batch_exception=None,
        expected_status=200,
        expected_results_count=1,
        expected_error_detail=None,
        expected_log_message="Forward batch completed",
        expected_log_level=logging.INFO,
    ),
    ForwardRequestsTestCase(
        name="multiple-urls-success",
        requests=[
            ("https://a.com/api", 200, {}),
            ("https://b.com/api", 201, {}),
        ],
        baggage=None,
        batch_exception=None,
        expected_status=200,
        expected_results_count=2,
        expected_error_detail=None,
        expected_log_message=None,
        expected_log_level=None,
    ),
    ForwardRequestsTestCase(
        name="single-url-4xx",
        requests=[("https://example.com/api", 404, {"error": "not found"})],
        baggage=None,
        batch_exception=None,
        expected_status=200,
        expected_results_count=1,
        expected_error_detail=None,
        expected_log_message=None,
        expected_log_level=None,
    ),
    ForwardRequestsTestCase(
        name="baggage-present",
        requests=[("https://example.com/api", 200, {})],
        baggage={"user-id": "123"},
        batch_exception=None,
        expected_status=200,
        expected_results_count=1,
        expected_error_detail=None,
        expected_log_message=None,
        expected_log_level=None,
    ),
    ForwardRequestsTestCase(
        name="baggage-absent",
        requests=[("https://example.com/api", 200, {})],
        baggage={},
        batch_exception=None,
        expected_status=200,
        expected_results_count=1,
        expected_error_detail=None,
        expected_log_message=None,
        expected_log_level=None,
    ),
    ForwardRequestsTestCase(
        name="batch-exception",
        requests=[("https://example.com/api", 200, {})],  # Mocked, won't execute
        baggage=None,
        batch_exception=RuntimeError,
        expected_status=500,
        expected_results_count=None,
        expected_error_detail="batch processing failed",
        expected_log_message="Batch processing failed",
        expected_log_level=logging.ERROR,
    ),
]


@pytest.mark.parametrize("case", FORWARD_REQUESTS_CASES, ids=lambda c: c.name)
@pytest.mark.asyncio
async def test_forward_requests(
    case: ForwardRequestsTestCase,
    async_client: AsyncClient,
    test_settings: Settings,
    mock_tracer: MagicMock,
    caplog: pytest.LogCaptureFixture,
) -> None:
    """forward_requests handles all scenarios comprehensively."""
    urls = [r[0] for r in case.requests]
    test_settings.forward_urls = urls

    with ExitStack() as stack:
        stack.enter_context(respx.mock)
        stack.enter_context(
            patch("app.routers.forward.trace.get_tracer", return_value=mock_tracer)
        )

        if case.baggage is not None:
            stack.enter_context(
                patch("app.routers.forward.baggage.get_all", return_value=case.baggage)
            )

        if case.batch_exception is not None:
            stack.enter_context(
                patch(
                    "app.routers.forward._forward_batch",
                    side_effect=case.batch_exception("boom"),
                )
            )

        stack.enter_context(
            caplog.at_level(
                case.expected_log_level or logging.INFO, logger="app.routers.forward"
            )
        )

        if case.batch_exception is None:
            for url, status, json_body in case.requests:
                respx.get(url).mock(return_value=httpx.Response(status, json=json_body))

        response = await async_client.get("/forward")

    assert response.status_code == case.expected_status

    if case.expected_results_count is not None:
        data = response.json()
        assert len(data["results"]) == case.expected_results_count

        for i, (url, status, _) in enumerate(case.requests):
            assert data["results"][i]["url"] == url
            assert data["results"][i]["status_code"] == status
            # Verify response_model_exclude_none=True: error field not in JSON
            assert "error" not in data["results"][i]
    elif case.expected_error_detail is not None:
        data = response.json()
        assert data["detail"]["error"] == case.expected_error_detail

    if case.baggage is not None:
        calls = mock_tracer.start_as_current_span.call_args_list
        batch_call = next((c for c in calls if c[0][0] == "forward.batch"), None)
        assert batch_call is not None
        attrs = batch_call[1].get("attributes", {})
        if case.baggage:
            assert "baggage" in attrs
        else:
            assert "baggage" not in attrs

    if case.expected_log_message is not None:
        assert any(
            case.expected_log_message in record.message for record in caplog.records
        )
    if case.expected_log_level is not None:
        assert any(
            record.levelno == case.expected_log_level for record in caplog.records
        )
