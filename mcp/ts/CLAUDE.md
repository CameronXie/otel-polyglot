# CLAUDE.md

TypeScript MCP server exposing API service endpoints as AI tooling. Part of a polyglot
OTel demo project.

## Quick Commands

```bash
make install    # Install dependencies
make build      # Compile TypeScript to dist/
make run        # Run the MCP server (stdio transport)
make run-http   # Run the MCP server (streamable-http transport)
make test       # Run tests
make lint       # Check formatting
make format     # Format code with prettier
make ci         # All CI checks
```

## Project Structure

| File/Directory                    | Purpose                                           |
|-----------------------------------|---------------------------------------------------|
| `src/index.ts`                    | Entry point: MCP server + OTel + tool reg         |
| `src/config.ts`                   | Env var configuration with zod                    |
| `src/otel.ts`                     | OTel SDK init (traces + metrics + logs)           |
| `src/transport.ts`                | Transport factory (stdio and streamable-http)     |
| `src/tools/tools.ts`              | `ToolBuilder`, `BuiltTool`, `jsonResult`, helpers |
| `src/tools/health-check.ts`       | health_check tool                                 |
| `src/tools/forward-request.ts`    | forward_request tool                              |
| `src/tools/list-services.ts`      | list_services tool                                |
| `src/tools/check-connectivity.ts` | check_connectivity tool                           |
| `tests/transport.test.ts`         | Transport layer tests                             |
| `tests/tools/`                    | Vitest test files matching src/tools              |

## OTel Instrumentation Pattern

| Layer         | Create Spans? | Rationale                        |
|---------------|---------------|----------------------------------|
| Tool Handlers | Manual        | Business logic like health_check |
| HTTP Client   | Auto          | fetch spans via OTel SDK         |

## Tool Registration

All tools are defined using the `ToolBuilder` class from `src/tools/tools.ts`:
`new ToolBuilder(name, description).input(schema).handler(...)`.
The `.input(schema)` step is optional (for tools that take no parameters). The builder
correlates the zod schema with the handler's input type, so handler parameters are
fully typed without manual annotation. Each built tool exposes a `register(server)` method
that calls `server.registerTool()` with the correct overload. `src/index.ts` iterates
over a tools array and calls `tool.register(server)` for each one. Add new tools by creating
a file in `src/tools/`, using `new ToolBuilder()`, and adding the factory call to the tools
array in `src/index.ts`.

Use `jsonResult(data)` and `jsonResult(data, true)` for success/error responses.
Use `PROBE_TIMEOUT_MS` and `REQUEST_TIMEOUT_MS` constants for fetch timeouts.

## Testing Conventions

- Use Vitest for all tests
- Mock external HTTP calls (fetch) in unit tests
- One test file per tool, matching the source structure
- Prefer `it.each` for table-driven tests when multiple cases share the same structure
- Test project code, not library behavior — e.g. don't test zod's `.default()` or
  third-party type narrowing; only test logic we own

## Comment Guidelines

- Use `/** */` doc comments on exported functions, types, and interfaces; use `//`
  for implementation notes
- Explain "why", not "what" — the code and types already show what
- Omit `@param` / `@returns` when the name and type make the meaning obvious
- No redundant comments that restate the code
- `@ts-expect-error` and lint suppressions: comment the reason on the same line

## Code Style

- **Prettier** for formatting
- **ES2024** target with strict TypeScript
- ESM modules throughout (`"type": "module"`)
- Extract helpers for clarity or testability, even if called only once — avoid
  nesting complex logic inline when a named function communicates intent better

## Git Workflow

- Conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Run `make ci` before pushing
