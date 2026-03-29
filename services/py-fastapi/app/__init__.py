"""FastAPI service with OpenTelemetry instrumentation."""

from importlib.metadata import PackageNotFoundError, version

try:
    __version__ = version("py-fastapi")
except PackageNotFoundError:
    __version__ = "dev"
