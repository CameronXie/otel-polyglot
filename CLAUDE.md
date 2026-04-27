# CLAUDE.md

Polyglot OpenTelemetry project. Each service under `services/` implements identical
endpoints and telemetry signals in a different language/framework.

## Quick Commands (root)

```
make up SERVICES=go-gin     # Start observability stack + service
make down                   # Stop all containers
make ci                     # Run CI checks for all services in Docker
make ci-go-gin              # Run go-gin CI checks in Docker
make build                  # Build all service Docker images
make docker-build-go-gin    # Build go-gin Docker image (ARCH=amd64 for x86_64)
```

## Project Layout

| Directory     | Purpose                                  |
|---------------|------------------------------------------|
| `services/`   | Service implementations (one per folder) |
| `otel/`       | OTel Collector configuration             |
| `prometheus/` | Prometheus configuration                 |
| `certs/`      | Auto-generated TLS certs (via Makefile)  |

## Service Contract

All services must implement:

- `GET /health`, `GET /forward`
- Traces: auto-instrumented HTTP spans + `forward.batch`, `forward.request`
- Metrics: `forward.requests` (counter), `forward.duration` (histogram)
- Logs: structured logs exported via OTLP with trace correlation

## README Guidelines

When writing or updating README files, follow these rules to keep documentation
consistent across the project.

### Structure

**Root README** owns shared content that applies to all services:

- Architecture diagram, service specification, endpoint payloads, telemetry details
- Observability stack, project structure, local development, TLS/certificates

**Service README** covers only implementation-specific details:

- Getting started (run locally, Docker, Docker Compose)
- Configuration reference (flags, env vars, OTLP env vars)
- Project structure (source files)
- Development guide (requirements, make targets, version injection)

Do not duplicate endpoint payloads, telemetry signal definitions, or troubleshooting
entries in service READMEs. Link back to the root README instead. Add a
service-level Troubleshooting section only when there are issues unique to that
implementation that aren't covered by the root README.

### Tone and Style

- Write in a neutral, technical-documentation tone — clear and direct
- Every section must open with 1–2 sentences explaining what the section contains and
  why it matters; avoid vague intros like "Commands for local development using Docker"
- Use consistent heading hierarchy: `##` for top-level sections, `###` for sub-sections
- Use action-oriented sub-headers (e.g., "Run Locally" not "Local Development")
- Wrap lines at ~80–90 characters for readable diffs

### Tables

- Use Markdown tables for structured reference data (endpoints, config, signals, files)
- Keep related items in one table when the row count is small (< 10 rows);
  split into separate tables only when categories are large enough to warrant it

### Code Blocks

- Always include the language identifier (```bash, ```json, ```mermaid)
- Add inline comments to explain non-obvious flags or arguments
- Use placeholders like `<service>` for variable values

### Cross-References

- Verify anchor links match the actual heading text (GitHub auto-generates anchors from
  heading text in lowercase with hyphens)
- Service READMEs link to root with relative paths: `../../README.md#section-name`

### Diagrams

- Use Mermaid (`\`\`\`mermaid`) for architecture diagrams
- Label nodes generically (e.g., "Service Implementation A") rather than naming specific
  services, so the diagram stays accurate as new implementations are added

## Conventions

- Conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Each service has its own `CLAUDE.md` with language-specific guidance
- Run `make ci` in the service directory before pushing changes
