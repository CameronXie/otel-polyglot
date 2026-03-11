# CLAUDE.md

Go-Gin API demonstrating **OpenTelemetry best practices**. Part of a polyglot OTel demo.

## Quick Commands
```
make build    # Build binary
make run      # Run on port 8080
make test     # Tests with coverage + race detection
make lint     # golangci-lint
make ci       # All CI checks
```

## Key Files
| File | Responsibility |
|------|----------------|
| `main.go` | Entry point, server lifecycle, OTel init |
| `handler.go` | Handler struct, HTTP client setup |
| `forward_handler.go` | Forward logic with spans/metrics |
| `otel.go` | OTel provider initialization |
| `config.go` | Configuration loading and validation |
| `metrics.go` | Metric definitions |

## OTel Instrumentation Pattern
| Layer | Create Spans? | Rationale |
|-------|---------------|-----------|
| HTTP Handler | ✅ Auto | Framework middleware (`otelgin`) |
| Business Logic | ✅ Manual | Key operations like `forward.batch` |
| Pure Functions | ❌ No | "When in doubt, don't instrument" |

## Error Handling (Dual Return)
```go
type ForwardResult struct {
    Error string `json:"error"`  // Safe: "operation failed"
}
// Internal: return fmt.Errorf("context: %w", err)  // Detailed
```

## Testing Conventions
- Use `map[string]struct{}` for table-driven tests (named cases)
- Use `t.Helper()` in assertion helper functions

### When to Merge vs Separate
- **Merge**: Same function, varying inputs → table-driven
- **Separate**: Different functions, tests needing isolation, or when test cases need mostly different struct fields

## Comment Guidelines
- Exported functions/types: doc comment starting with name
- Explain "why", not "what" (code shows what)
- nolint comments: explain the reason
- No obvious/redundant comments

## Git Workflow
- Conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Run `make ci` before pushing
