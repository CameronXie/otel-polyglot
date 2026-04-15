"""OpenTelemetry SDK initialization for traces, metrics, and logs."""

from __future__ import annotations

import logging
from collections.abc import Iterator
from contextlib import contextmanager
from typing import TYPE_CHECKING

from opentelemetry import metrics, trace
from opentelemetry._logs import set_logger_provider
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import OTLPLogExporter
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from opentelemetry.instrumentation.logging.handler import LoggingHandler
from opentelemetry.sdk._logs import LoggerProvider
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.semconv.attributes.service_attributes import (
    SERVICE_NAME,
    SERVICE_VERSION,
)

if TYPE_CHECKING:
    from fastapi import FastAPI

    from app.config import Settings


def create_resource(settings: Settings) -> Resource:
    return Resource.create(
        {
            SERVICE_NAME: settings.get_service_name(),
            SERVICE_VERSION: __import__("app").__version__,
        }
    )


def init_tracing(resource: Resource) -> TracerProvider:
    exporter = OTLPSpanExporter()
    provider = TracerProvider(resource=resource)
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    return provider


def init_metrics(resource: Resource) -> MeterProvider:
    exporter = OTLPMetricExporter()
    reader = PeriodicExportingMetricReader(exporter)
    provider = MeterProvider(
        resource=resource,
        metric_readers=[reader],
    )
    metrics.set_meter_provider(provider)
    return provider


def init_logging(resource: Resource, log_level: str = "INFO") -> LoggerProvider:
    exporter = OTLPLogExporter()
    provider = LoggerProvider(resource=resource)
    provider.add_log_record_processor(BatchLogRecordProcessor(exporter))
    set_logger_provider(provider)

    level = getattr(logging, log_level)

    # Attach OTel handler to root logger for trace correlation
    handler = LoggingHandler(level=level, logger_provider=provider)

    # Set root logger level
    logging.getLogger().setLevel(level)
    logging.getLogger().addHandler(handler)

    return provider


@contextmanager
def init_otel(settings: Settings) -> Iterator[None]:
    """Initialize OpenTelemetry SDK with graceful shutdown."""
    resource = create_resource(settings)

    tracer_provider = init_tracing(resource)
    meter_provider = init_metrics(resource)
    logger_provider = init_logging(resource, settings.log_level)

    # Instrument HTTPX for automatic client spans
    HTTPXClientInstrumentor().instrument()

    try:
        yield
    finally:
        # Shutdown providers gracefully
        tracer_provider.shutdown()
        meter_provider.shutdown()
        logger_provider.shutdown()


def instrument_fastapi(app: FastAPI) -> None:
    """Instrument FastAPI app for automatic request spans."""
    FastAPIInstrumentor.instrument_app(app)
