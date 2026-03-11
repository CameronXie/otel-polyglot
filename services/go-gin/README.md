# Go Gin Service

Go API service built with the [Gin](https://github.com/gin-gonic/gin) framework and
instrumented with OpenTelemetry for traces, metrics, and logs. Implements the
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
GO_GIN_FORWARD_URLS="https://httpbin.org/get,https://example.com" make run

# Or with flags
go run . --port 8080 --forward-urls "https://httpbin.org/get"
```

### Run with Docker

```bash
# Build the image (default: arm64, use ARCH=amd64 for x86_64)
make docker-build

# Run the container
docker run -p 8080:8080
-e OTEL_EXPORTER_OTLP_ENDPOINT=https://collector:4317
-e GO_GIN_SERVICE_NAME=go-gin
-e GO_GIN_FORWARD_URLS="https://httpbin.org/get"
go-gin:latest
```

### Run with Docker Compose

```bash
# From project root — starts the observability stack and this service
make up PROFILES=go-gin
```

## Configuration Reference

The service accepts configuration through CLI flags and environment variables.
`GO_GIN_*` environment variables take precedence over their `OTEL_*` equivalents when
both are set.

| Option       | Flag             | Environment Variable                         | Default           | Description                               |
|--------------|------------------|----------------------------------------------|-------------------|-------------------------------------------|
| Port         | `--port`         | `GO_GIN_PORT`                                | `8080`            | Server listen port                        |
| Forward URLs | `--forward-urls` | `GO_GIN_FORWARD_URLS`                        | None              | Comma-separated URLs for forward endpoint |
| Service Name | `--service-name` | `GO_GIN_SERVICE_NAME` or `OTEL_SERVICE_NAME` | `unknown_service` | OpenTelemetry resource service name       |
| Environment  | `--environment`  | `GO_GIN_ENVIRONMENT`                         | `development`     | Deployment environment identifier         |

### OTLP Environment Variables

Standard OpenTelemetry SDK environment variables control exporter behaviour. These are
independent of the application-specific variables above.

| Variable                      | Default                 | Description                                                 |
|-------------------------------|-------------------------|-------------------------------------------------------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OTLP collector endpoint (gRPC)                              |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false`                 | Disable TLS verification for collector connection           |
| `OTEL_SERVICE_NAME`           | `unknown_service`       | Fallback service name when `GO_GIN_SERVICE_NAME` is not set |
| `OTEL_RESOURCE_ATTRIBUTES`    | —                       | Additional resource attributes as `key=value` pairs         |

## Project Structure

The service is a single Go module with no sub-packages. Each source file has a
corresponding `_test.go` file where applicable.

| File                     | Purpose                                           |
|--------------------------|---------------------------------------------------|
| `main.go`                | Entrypoint, server lifecycle, graceful shutdown   |
| `config.go`              | Configuration loading via flags and environment   |
| `handler.go`             | Handler struct and HTTP client setup              |
| `forward_handler.go`     | `/forward` endpoint with concurrent fan-out logic |
| `healthcheck_handler.go` | `/health` endpoints                               |
| `otel.go`                | OpenTelemetry SDK initialisation (all signals)    |
| `metrics.go`             | Custom metric instrument definitions              |
| `version.go`             | Build-time version injection via `-ldflags`       |

## Development Guide

Instructions for building, testing, and linting the service outside of Docker.

### Requirements

- **Go 1.26+**
- **golangci-lint v2.10+** — required for `make lint`
- **govulncheck** — required for `make security`

### Make Targets

```bash
make build         # Build binary to dist/go-gin
make docker-build  # Build Docker image (default: arm64, use ARCH=amd64 for x86_64)
make run           # Run the service on port 8080
make test          # Run tests with coverage and race detection
make lint          # Format code and run linters
make security      # Run linters and govulncheck
make ci            # Run all CI checks (test + security)
make clean         # Remove build artifacts
```

Test coverage reports are generated at `dist/coverage.html`.

### Version Injection

The binary version is set at build time using `-ldflags`. Running `make build` handles
this automatically. When running from source without ldflags, the version defaults to
`dev`.

```bash
go build -ldflags "-X main.Version=1.0.0" -o dist/go-gin .
```
