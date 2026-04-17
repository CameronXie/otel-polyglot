# TypeScript MCP Server

TypeScript MCP (Model Context Protocol) server that exposes API service endpoints as
AI tooling. Built with the MCP SDK and instrumented with OpenTelemetry for traces,
metrics, and logs.

The server supports two transport modes: **stdio** (default, for CLI integration) and
**streamable-http** (for HTTP-based clients). It connects to the same observability
stack as the API services and emits the
[MCP telemetry signals](../../README.md#mcp-telemetry-signals) defined in the project
root.

## Getting Started

The server can be run locally, in a standalone Docker container, or as part of the
full observability stack via Docker Compose.

### Run Locally

```bash
# Install dependencies
make install

# Run with stdio transport (default)
make run

# Run with streamable-http transport
make run-http

# Run with custom configuration
TS_MCP_SERVICE_URLS="go-gin=http://localhost:8080" make run
TS_MCP_TRANSPORT=streamable-http TS_MCP_PORT=8080 make run
```

### Run with Docker

```bash
# Build the image (default: amd64, use ARCH=arm64 for ARM)
make docker-build

# Run with stdio transport (default)
docker run \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=https://collector:4317 \
  -e TS_MCP_SERVICE_NAME=ts-mcp \
  -e TS_MCP_SERVICE_URLS="go-gin=http://otel_polyglot_go_gin:8080" \
  ts-mcp:latest

# Run with streamable-http transport
docker run \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=https://collector:4317 \
  -e TS_MCP_SERVICE_NAME=ts-mcp \
  -e TS_MCP_SERVICE_URLS="go-gin=http://otel_polyglot_go_gin:8080" \
  -e TS_MCP_TRANSPORT=streamable-http \
  -e TS_MCP_PORT=8080 \
  -p 8080:8080 \
  ts-mcp:latest
```

### Run with Docker Compose

```bash
# From project root — starts the observability stack and this server
make up PROFILES=ts-mcp
```

## Configuration Reference

The server accepts configuration through environment variables.

| Option         | Environment Variable    | Default       | Description                                                       |
|----------------|-------------------------|---------------|-------------------------------------------------------------------|
| Service Name   | `TS_MCP_SERVICE_NAME`   | `ts-mcp`      | OpenTelemetry resource service name                               |
| Service URLs   | `TS_MCP_SERVICE_URLS`   | None          | Comma-separated `name=url` pairs for services                     |
| Log Level      | `TS_MCP_LOG_LEVEL`      | `INFO`        | Logging level (DEBUG, INFO, WARN, ERROR)                          |
| Deployment Env | `TS_MCP_DEPLOYMENT_ENV` | `development` | OTel resource `deployment.environment` (falls back to `NODE_ENV`) |
| Transport      | `TS_MCP_TRANSPORT`      | `stdio`       | Transport mode: `stdio` or `streamable-http`                      |
| Port           | `TS_MCP_PORT`           | `8080`        | Listen port (streamable-http only)                                |
| Host           | `TS_MCP_HOST`           | `127.0.0.1`   | Listen host (streamable-http only)                                |

`TS_MCP_SERVICE_URLS` format: `name1=http://host:port,name2=http://host:port`

### OTLP Environment Variables

Standard [OpenTelemetry environment variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
control exporter behaviour (endpoints, TLS, headers, etc.). These are read
automatically by the OTel SDK — no application-level configuration is needed.

## Project Structure

Source files and directories in the TypeScript MCP server, with a summary of each
file's responsibility.

| File/Directory                    | Purpose                                             |
|-----------------------------------|-----------------------------------------------------|
| `src/index.ts`                    | Entry point: MCP server + OTel init                 |
| `src/config.ts`                   | Zod-validated env var configuration                 |
| `src/otel.ts`                     | OpenTelemetry SDK initialization                    |
| `src/transport.ts`                | Transport factory (stdio and streamable-http)       |
| `src/tools/tools.ts`              | ToolBuilder, registration helpers, shared utilities |
| `src/tools/health-check.ts`       | health_check tool                                   |
| `src/tools/forward-request.ts`    | forward_request tool                                |
| `src/tools/list-services.ts`      | list_services tool                                  |
| `src/tools/check-connectivity.ts` | check_connectivity tool                             |
| `tests/tools/`                    | Vitest test files                                   |

## Available Tools

MCP tools exposed by the server, along with their input parameters. Tools are
registered automatically at startup from `src/index.ts`.

| Tool                 | Inputs                                                                                                                                                       | Description                                                  |
|----------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------|
| `list_services`      | None                                                                                                                                                         | List all configured backend services and their URLs          |
| `health_check`       | `service` (string, required)                                                                                                                                 | Check service health via its `/health` endpoint              |
| `check_connectivity` | `service` (string, required)                                                                                                                                 | Check network connectivity via HEAD request, returns latency |
| `forward_request`    | `service` (string, required), `method` (enum, default: `GET`), `path` (string, default: `/forward`), `headers` (record, optional), `body` (string, optional) | Forward an HTTP request to a configured service              |

## Registering with Claude Code

Steps to register the MCP server with Claude Code so it can be used as an AI tooling
provider.

```bash
make install && make build

# Register with stdio transport (default)
TS_MCP_SERVICE_URLS="go-gin=http://localhost:8080" make mcp-add

# Register with streamable-http transport
TRANSPORT=streamable-http make mcp-add

claude mcp list
```

## Development Guide

Instructions for building, testing, and linting outside of Docker.

### Requirements

- **Node.js 24+**
- **pnpm**
- **make**

### Make Targets

```bash
make install       # Install dependencies
make build         # Compile TypeScript to dist/
make docker-build  # Build Docker image (default: amd64, use ARCH=arm64 for ARM)
make run           # Run the MCP server (stdio transport)
make run-http      # Run the MCP server (streamable-http transport)
make test          # Run tests with coverage
make format        # Format code with prettier
make lint          # Check code formatting
make ci            # Run all CI checks (format-check, lint, build, test)
make mcp-add       # Register this MCP server with Claude Code (TRANSPORT=stdio|streamable-http)
make clean         # Remove build artifacts
```
