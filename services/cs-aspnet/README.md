# C# ASP.NET Core Service

C# API service built with [ASP.NET Core](https://learn.microsoft.com/en-us/aspnet/core/)
and instrumented with OpenTelemetry for traces, metrics, and logs. Implements the
[service specification](../../README.md#service-specification) and emits all required
[telemetry signals](../../README.md#service-telemetry-signals).

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
CS_ASPNET_FORWARD_URLS="https://httpbin.org/get,https://example.com" make run

# Or directly with dotnet
dotnet run --launch-profile http
```

### Run with Docker

```bash
# Build the image (default: amd64, use ARCH=arm64 for ARM)
make docker-build

# Run the container
docker run -p 8080:8080 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=https://collector:4317 \
  -e CS_ASPNET_SERVICE_NAME=cs-aspnet \
  -e CS_ASPNET_FORWARD_URLS="https://httpbin.org/get" \
  cs-aspnet:latest
```

### Run with Docker Compose

```bash
# From project root — starts the observability stack and this service
make up SERVICES=cs-aspnet
```

## Configuration Reference

The service accepts configuration through environment variables. `CS_ASPNET_*` variables
take precedence over their `OTEL_*` equivalents when both are set.

| Option       | Environment Variable                            | Default           | Description                                 |
|--------------|-------------------------------------------------|-------------------|---------------------------------------------|
| Port         | `CS_ASPNET_PORT`                                | `8080`            | Server listen port                          |
| Forward URLs | `CS_ASPNET_FORWARD_URLS`                        | None              | Comma-separated URLs for forward endpoint   |
| Service Name | `CS_ASPNET_SERVICE_NAME` or `OTEL_SERVICE_NAME` | `unknown_service` | OpenTelemetry resource service name         |
| Log Level    | `CS_ASPNET_LOG_LEVEL`                           | `info`            | Logging level (debug, info, warning, error) |

### OTLP Environment Variables

Standard OpenTelemetry SDK environment variables control exporter behaviour. These are
independent of the application-specific variables above.

| Variable                      | Default                 | Description                                                    |
|-------------------------------|-------------------------|----------------------------------------------------------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OTLP collector endpoint (gRPC)                                 |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false`                 | Disable TLS verification for collector connection              |
| `OTEL_SERVICE_NAME`           | `unknown_service`       | Fallback service name when `CS_ASPNET_SERVICE_NAME` is not set |
| `OTEL_RESOURCE_ATTRIBUTES`    | —                       | Additional resource attributes as `key=value` pairs            |

## Project Structure

The service uses ASP.NET Core with minimal APIs for trivial endpoints and controllers
for complex logic. Business logic lives in service classes registered via DI.

| File/Directory                | Purpose                                        |
|-------------------------------|------------------------------------------------|
| `CsAspNet.slnx`               | Solution file tying src and tests              |
| `src/CsAspNet/Program.cs`     | Entry point, DI, middleware, endpoint maps     |
| `src/CsAspNet/Configuration/` | Options classes bound to env vars              |
| `src/CsAspNet/Controllers/`   | API controllers (`[ApiController]`)            |
| `src/CsAspNet/Models/`        | Request/response record types                  |
| `src/CsAspNet/Services/`      | Business logic (forward service)               |
| `src/CsAspNet/Telemetry/`     | OTel SDK setup, custom metrics, ActivitySource |
| `tests/CsAspNet.Tests/`       | Integration tests with WebApplicationFactory   |

## Development Guide

Instructions for building, testing, and linting the service outside of Docker.

### Requirements

- **.NET 10.0 SDK**
- **make**

### Make Targets

```bash
make build         # Publish application to artifacts/publish
make docker-build  # Build Docker image (default: amd64, use ARCH=arm64 for ARM)
make docker-publish # Build and push Docker image to registry
make run           # Run the service on port 8080
make test          # Run tests with coverage
make format        # Format code with dotnet format
make lint          # Run linters (dotnet format check + style analysis)
make security      # Check for vulnerable NuGet packages
make ci            # Run all CI checks (lint, security, test)
make clean         # Remove build artifacts
```

Test coverage reports are generated at `artifacts/`.

### Version Injection

The assembly version is set at build time via `.csproj` properties. Running `make build`
handles this automatically.
