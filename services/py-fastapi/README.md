# Python FastAPI Service

Python API service built with [FastAPI](https://fastapi.tiangolo.com/) and instrumented
with OpenTelemetry for traces, metrics, and logs. Implements the
[service specification](../../README.md#service-specification) and emits all required
[telemetry signals](../../README.md#telemetry-signals).

Telemetry data is exported via gRPC to an OTLP-compatible collector. Context propagation
supports W3C Trace Context and Baggage.

## Getting Started

The service can be run locally, in a standalone Docker container, or as part of the
full observability stack via Docker Compose.

### Run Locally

```bash
# Run with default configuration
make run

# Run with custom configuration
PY_FASTAPI_FORWARD_URLS="https://httpbin.org/get,https://example.com" make run

# Or directly with uvicorn
uvicorn app.main:app --port 8080
```

### Run with Docker

```bash
# Build the image (default: amd64, use ARCH=arm64 for ARM)
make docker-build

# Run the container
docker run -p 8080:8080 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=https://collector:4317 \
  -e PY_FASTAPI_SERVICE_NAME=py-fastapi \
  -e PY_FASTAPI_FORWARD_URLS="https://httpbin.org/get" \
  py-fastapi:latest
```

### Run with Docker Compose

```bash
# From project root — starts the observability stack and this service
make up PROFILES=py-fastapi
```

## Configuration Reference

The service accepts configuration through environment variables. `FASTAPI_*` variables
take precedence over their `OTEL_*` equivalents when both are set.

| Option       | Environment Variable                             | Default           | Description                                           |
|--------------|--------------------------------------------------|-------------------|-------------------------------------------------------|
| Port         | `PY_FASTAPI_PORT`                                | `8080`            | Server listen port                                    |
| Forward URLs | `PY_FASTAPI_FORWARD_URLS`                        | None              | Comma-separated URLs for forward endpoint             |
| Service Name | `PY_FASTAPI_SERVICE_NAME` or `OTEL_SERVICE_NAME` | `unknown_service` | OpenTelemetry resource service name                   |
| Log Level    | `PY_FASTAPI_LOG_LEVEL`                           | `info`            | Logging level (debug, info, warning, error, critical) |

### OTLP Environment Variables

Standard OpenTelemetry SDK environment variables control exporter behaviour. These are
independent of the application-specific variables above.

| Variable                      | Default                 | Description                                                     |
|-------------------------------|-------------------------|-----------------------------------------------------------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OTLP collector endpoint (gRPC)                                  |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false`                 | Disable TLS verification for collector connection               |
| `OTEL_SERVICE_NAME`           | `unknown_service`       | Fallback service name when `PY_FASTAPI_SERVICE_NAME` is not set |
| `OTEL_RESOURCE_ATTRIBUTES`    | —                       | Additional resource attributes as `key=value` pairs             |

## Project Structure

The service follows FastAPI's recommended structure using `APIRouter` for modular
routing and Pydantic for configuration and data validation.

| File/Directory               | Purpose                              |
|------------------------------|--------------------------------------|
| `app/main.py`                | FastAPI app factory, entry point     |
| `app/config.py`              | Pydantic settings from environment   |
| `app/otel.py`                | OpenTelemetry SDK initialization     |
| `app/metrics.py`             | Custom metric definitions            |
| `app/routers/`               | APIRouter modules for endpoints      |
| `app/routers/healthcheck.py` | GET /health                          |
| `app/routers/forward.py`     | GET /forward with OTel spans/metrics |
| `app/models/`                | Pydantic request/response models     |
| `tests/`                     | pytest tests matching app structure  |

## Development Guide

Instructions for building, testing, and linting the service outside of Docker.

### Requirements

- **Python 3.11+**
- **make**

### Make Targets

```bash
make install       # Install dependencies in virtual environment
make build         # Build package to dist/
make docker-build  # Build Docker image (default: amd64, use ARCH=arm64 for ARM)
make run           # Run the service on port 8080
make test          # Run tests with coverage
make format        # Format code with ruff
make lint          # Run linters (ruff + mypy)
make security      # Run pip-audit for security scanning
make ci            # Run all CI checks (format-check, lint, security, test)
make clean         # Remove build artifacts
```

Test coverage reports are generated at `dist/coverage.html`.

### Version Injection

The package version is read from `pyproject.toml` at runtime via
`importlib.metadata.version()`. No build-time injection is required.

### Code Style

- **Ruff** for linting and formatting (replaces black, isort, flake8)
- **mypy** with strict mode for type checking
- Type hints required on all function signatures
