"""Forward endpoint with OpenTelemetry instrumentation."""

from __future__ import annotations

import asyncio
import logging
import time
from typing import TYPE_CHECKING

import httpx
from fastapi import APIRouter, HTTPException
from opentelemetry import baggage, trace
from opentelemetry.semconv.attributes.http_attributes import (
    HTTP_REQUEST_METHOD,
    HTTP_RESPONSE_STATUS_CODE,
)
from opentelemetry.semconv.attributes.server_attributes import SERVER_ADDRESS
from opentelemetry.semconv.attributes.url_attributes import URL_FULL, URL_SCHEME

from app.config import SettingsDep
from app.metrics import ForwardMetrics, get_forward_metrics
from app.models.forward import ErrorResponse, ForwardResponse, ForwardResult

if TYPE_CHECKING:
    from opentelemetry.trace import Tracer


# HTTP client timeout in seconds
HTTP_TIMEOUT = 2.0

# Maximum response body size (1 MB)
MAX_RESPONSE_SIZE = 1 << 20

router = APIRouter(tags=["forward"])
logger = logging.getLogger(__name__)


@router.get(
    "/forward",
    response_model=ForwardResponse,
    response_model_exclude_none=True,
    responses={500: {"model": ErrorResponse}},
)
async def forward_requests(settings: SettingsDep) -> ForwardResponse:
    """Forward GET requests to all configured URLs in parallel."""
    ctx = baggage.get_all()
    tracer: Tracer = trace.get_tracer(__name__)

    # Build span attributes
    span_attrs = {
        "forward.urls": str(settings.forward_urls),
    }
    if ctx:
        span_attrs["baggage"] = str(ctx)

    with tracer.start_as_current_span(
        "forward.batch",
        attributes=span_attrs,
        kind=trace.SpanKind.INTERNAL,
    ) as batch_span:
        logger.info(
            "Starting forward batch",
            extra={"url.count": len(settings.forward_urls), "baggage": str(ctx)},
        )

        try:
            results = await _forward_batch(settings.forward_urls, tracer)
        except Exception as e:
            logger.exception("Batch processing failed", extra={"error": str(e)})
            batch_span.record_exception(e)
            batch_span.set_status(trace.Status(trace.StatusCode.ERROR, "batch failed"))
            raise HTTPException(
                status_code=500,
                detail=ErrorResponse(error="batch processing failed").model_dump(),
            )

        batch_span.set_attribute("forward.batch_size", len(settings.forward_urls))
        batch_span.set_status(trace.Status(trace.StatusCode.OK))
        logger.info("Forward batch completed", extra={"results.count": len(results)})

        return ForwardResponse(results=results)


async def _forward_batch(urls: list[str], tracer: Tracer) -> list[ForwardResult]:
    metrics_instance = get_forward_metrics()

    async with httpx.AsyncClient(timeout=HTTP_TIMEOUT) as client:
        tasks = [_forward_single(url, client, tracer, metrics_instance) for url in urls]
        results = await asyncio.gather(*tasks)

    return list(results)


async def _forward_single(
    url: str,
    client: httpx.AsyncClient,
    tracer: Tracer,
    metrics_instance: ForwardMetrics,
) -> ForwardResult:
    start_time = time.perf_counter()

    with tracer.start_as_current_span(
        "forward.request",
        attributes={URL_FULL: url},
        kind=trace.SpanKind.CLIENT,
    ) as span:
        try:
            async with client.stream("GET", url) as response:
                duration = time.perf_counter() - start_time

                # Record metrics
                parsed_url = httpx.URL(url)
                metrics_instance.record_request(
                    server_address=parsed_url.host,
                    url_scheme=parsed_url.scheme,
                    duration=duration,
                )

                # Update span attributes
                span.set_attributes(
                    {
                        HTTP_RESPONSE_STATUS_CODE: response.status_code,
                        HTTP_REQUEST_METHOD: "GET",
                        SERVER_ADDRESS: parsed_url.host,
                        URL_SCHEME: parsed_url.scheme,
                    }
                )

                if response.status_code >= 400:
                    span.set_status(
                        trace.Status(trace.StatusCode.ERROR, "upstream error")
                    )
                    logger.warning(
                        "Upstream returned error status",
                        extra={"http.response.status_code": response.status_code},
                    )
                else:
                    span.set_status(trace.Status(trace.StatusCode.OK))
                    logger.info("Request completed successfully")

                # Stream and limit response body size
                body = await _read_limited_body(response, MAX_RESPONSE_SIZE)

                return ForwardResult(
                    url=url,
                    status_code=response.status_code,
                    body=body,
                    duration_seconds=duration,
                )

        except httpx.HTTPError as e:
            duration = time.perf_counter() - start_time
            span.record_exception(e)
            span.set_status(trace.Status(trace.StatusCode.ERROR, "request failed"))
            logger.error("Failed to execute request", extra={"error": str(e)})

            return ForwardResult(
                url=url,
                error=str(e),
                duration_seconds=duration,
            )

        except Exception as e:
            duration = time.perf_counter() - start_time
            span.record_exception(e)
            span.set_status(trace.Status(trace.StatusCode.ERROR, "unexpected error"))
            logger.exception(
                "Unexpected error in forward task", extra={"error": str(e)}
            )

            return ForwardResult(
                url=url,
                error=str(e),
                duration_seconds=duration,
            )


async def _read_limited_body(response: httpx.Response, max_size: int) -> str:
    """Read the response body up to max_size bytes."""
    chunks: list[bytes] = []
    remaining = max_size

    async for chunk in response.aiter_bytes():
        if remaining < len(chunk):
            chunks.append(chunk[:remaining])
            break

        remaining -= len(chunk)
        chunks.append(chunk)

    return b"".join(chunks).decode(response.encoding or "utf-8", errors="replace")
