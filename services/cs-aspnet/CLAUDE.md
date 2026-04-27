# CLAUDE.md

C# ASP.NET Core service demonstrating **OpenTelemetry best practices**. Part of a
polyglot OTel demo project.

## Quick Commands

```bash
make build         # Publish to artifacts/publish
make run           # Run on port 8080
make test          # Tests with coverage
make format        # Format with dotnet format
make lint          # dotnet format check + style analysis
make security      # dotnet audit
make ci            # All CI checks (lint, security, test)
```

## Project Structure

| File/Directory                | Purpose                                         |
|-------------------------------|-------------------------------------------------|
| `CsAspNet.slnx`               | Solution file tying src and tests               |
| `src/CsAspNet/Program.cs`     | Entry point, DI, middleware, endpoint maps      |
| `src/CsAspNet/Configuration/` | Options classes bound to env vars               |
| `src/CsAspNet/Controllers/`   | API controllers (`[ApiController]`)             |
| `src/CsAspNet/Models/`        | Request/response record types                   |
| `src/CsAspNet/Services/`      | Business logic (thin controllers, fat services) |
| `src/CsAspNet/Telemetry/`     | OTel SDK setup, custom metrics, ActivitySource  |
| `tests/CsAspNet.Tests/`       | Integration tests with WebApplicationFactory    |

## Architecture

- **Framework**: ASP.NET Core 10.0 with mixed minimal APIs and controllers
- **Pattern**: Minimal APIs for trivial endpoints (`/health`), controllers
  (`[ApiController]`) for complex endpoints (`/forward`). Services handle business logic.
- **HTTP Client**: Typed `HttpClient` via `AddHttpClient<T>()` with OTel
  auto-instrumentation
- **Configuration**: Options pattern (`IOptions<T>`) bound to `CS_ASPNET_*` env vars
- **Telemetry**: OpenTelemetry .NET SDK, OTLP gRPC exporter

## Project Conventions

- **Service root**: kebab-case (`services/cs-aspnet/`) for polyglot consistency —
  matches the convention used by Go, Python, and TypeScript services in this repo
- **Typed HTTP clients only**: Use `AddHttpClient<T>()` — do NOT use named clients
  (`AddHttpClient("name")`). Typed clients provide compile-time safety and eliminate
  magic strings
- **Env var mapping**: `PrefixedEnvVarConfigurationSource` maps flat `CS_ASPNET_*`
  env vars to the `CS_ASPNET:*` config hierarchy so options binding works. Add new
  env vars to the `Mapping` dictionary in
  `Configuration/PrefixedEnvVarConfigurationSource.cs`

## OTel Instrumentation

| Layer          | Spans? | Rationale                                   |
|----------------|--------|---------------------------------------------|
| HTTP Server    | Auto   | `AddAspNetCoreInstrumentation()` middleware |
| HTTP Client    | Auto   | `AddHttpClientInstrumentation()`            |
| Business Logic | Manual | `ActivitySource.StartActivity()`            |

### Custom Spans

Use `System.Diagnostics.ActivitySource` — .NET's built-in span API, no extra NuGet
package needed. Register the source with the SDK: `tracing.AddSource("CsAspNet")`.

- `forward.batch` — parent span covering the full batch (`ActivityKind.Internal`)
- `forward.request` — child span per forwarded URL (`ActivityKind.Client`)

Null-check the `Activity?` from `StartActivity()` — returns null when no listener is
subscribed.

### Custom Metrics

Use `System.Diagnostics.Metrics` via `IMeterFactory` (DI-aware, avoids static state).
Register the meter: `metrics.AddMeter("CsAspNet")`.

- `forward.requests` — `Counter<long>` with tags `server.address`, `url.scheme`
- `forward.duration` — `Histogram<double>` in seconds with same tags

Histogram buckets configured via `AddView("forward.duration", new ExplicitBucketHistogramConfiguration)`.

### Structured Logging

Trace/span correlation is automatic with `AddOpenTelemetry().WithLogging()` — no manual
enrichment needed.

### OTel Setup

Registration lives in `Telemetry/OtelExtensions.cs`. Called from `Program.cs`:

```csharp
builder.Services.AddOtel(builder.Configuration, serviceOptions);
```

Requires `using OpenTelemetry.Logs;` for the `WithLogging().AddOtlpExporter()` call.

## Testing

- Prefer table-driven tests with `[Theory]` + `[MemberData]` or `[InlineData]` for
  similar inputs or structured test cases
- `CustomWebApplicationFactory` clears OTel env vars to avoid cert/connection errors in CI
- Override `HttpClient` in tests by replacing the `AddHttpClient<T>()` registration with
  a stub `HttpMessageHandler` to avoid external HTTP dependencies
- `InternalsVisibleTo` in `.csproj` allows tests to access `internal` members

## NuGet Packages

Pin all `OpenTelemetry.*` packages to the same version to avoid runtime assembly
conflicts. Key packages:

- `OpenTelemetry.Extensions.Hosting`
- `OpenTelemetry.Exporter.OpenTelemetryProtocol`
- `OpenTelemetry.Instrumentation.AspNetCore`
- `OpenTelemetry.Instrumentation.Http`

## Version Injection

Assembly version is set at build time via `.csproj` properties (`<VersionPrefix>`).
Access at runtime via `Assembly.GetExecutingAssembly().GetName().Version`.

## Git Workflow

- Conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Run `make ci` before pushing
