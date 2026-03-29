# CLAUDE.md

Python FastAPI service demonstrating **OpenTelemetry best practices**. Part of a polyglot
OTel demo project.

## Quick Commands

```bash
make install    # Install dependencies in virtual environment
make run        # Run on port 8080
make test       # Tests with coverage
make lint       # ruff check + mypy
make format     # ruff format
make security   # pip-audit
make ci         # All CI checks
```

## Project Structure

| File/Directory             | Purpose                                    |
|----------------------------|--------------------------------------------|
| `app/main.py`              | FastAPI app factory, entry point           |
| `app/config.py`            | Pydantic settings from environment         |
| `app/dependencies.py`      | FastAPI dependencies (settings injection)  |
| `app/otel.py`              | OpenTelemetry SDK initialization           |
| `app/metrics.py`           | Custom metric definitions                  |
| `app/routers/`             | APIRouter modules for endpoints            |
| `app/routers/healthcheck.py` | GET /health                              |
| `app/routers/forward.py`   | GET /forward with OTel spans/metrics       |
| `app/models/`              | Pydantic request/response models           |
| `tests/`                   | pytest tests matching app structure        |

## Code Principles

### Prefer Library-Specific Solutions

When encountering unexpected behavior in a specific library, search for library-specific
features first before falling back to generic patterns. Generic workarounds often exist,
but library-specific solutions are usually cleaner and more idiomatic.

### Match the Solution to the Layer

Understand which layer controls the behavior you need, then use that layer's mechanisms.
Example: FastAPI controls HTTP response serialization via router parameters; Pydantic
controls data validation and model structure.

## OTel Instrumentation Pattern

| Layer            | Create Spans? | Rationale                               |
|------------------|---------------|-----------------------------------------|
| HTTP Handler     | Auto          | FastAPIInstrumentor middleware          |
| Business Logic   | Manual        | Key operations like `forward.batch`     |
| HTTP Client      | Auto          | HTTPXClientInstrumentor                 |

## Testing Conventions

- Use `@pytest.mark.asyncio` for async tests
- Use `respx` for mocking `httpx` requests
- Use `unittest.mock.patch` for dependency injection
- Use `@pytest.mark.parametrize` at module level for table-driven tests (no classes)
- Use `autouse` fixtures for common test setup (e.g., cache clearing)
- Use table-driven tests for similar setup with different inputs/outputs
- Use single test for identical setup checking multiple properties of same result
- Use dataclass for complex test case definitions
- Keep performance tests (e.g., timing) separate from correctness tests

## Code Style

- **Ruff** for linting and formatting (replaces black, isort, flake8)
- **mypy** with strict mode for type checking
- Type hints required on all function signatures
- Line length: 88 characters (black-compatible)

## Comment Guidelines

- Public functions/classes: docstring per PEP 257
- Explain "how to use" (docstrings) or "why" (comments), not "what"
- Skip when name and signature are self-explanatory

## Version Injection

Version is read from `pyproject.toml` via `importlib.metadata.version()` at runtime.
No build-time injection required.

## Git Workflow

- Conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Run `make ci` before pushing
