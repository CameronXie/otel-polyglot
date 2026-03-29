"""FastAPI application factory and entry point."""

from __future__ import annotations

import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from opentelemetry import trace

from app.config import get_settings
from app.otel import init_otel, instrument_fastapi
from app.routers import forward_router, healthcheck_router

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(_app: FastAPI) -> AsyncGenerator[None]:
    """Manage application lifecycle with OTel initialization."""
    settings = get_settings()
    tracer = trace.get_tracer(__name__)

    with tracer.start_as_current_span("app.startup"):
        logger.info(
            "Starting service",
            extra={
                "port": settings.port,
                "service_name": settings.get_service_name(),
            },
        )

    with init_otel(settings):
        yield

    with tracer.start_as_current_span("app.shutdown"):
        logger.info("Service shutdown complete")


def create_app() -> FastAPI:
    app = FastAPI(
        title="py-fastapi",
        description="FastAPI service with OpenTelemetry instrumentation",
        version=__import__("app").__version__,
        lifespan=lifespan,
    )

    app.include_router(healthcheck_router)
    app.include_router(forward_router)

    # Instrument for automatic HTTP spans
    instrument_fastapi(app)

    return app


# Application instance for uvicorn
app = create_app()


if __name__ == "__main__":
    # Local development entry point with hot reload
    import uvicorn

    uvicorn.run("app.main:app", host="0.0.0.0", port=8080, reload=True)
