"""Application configuration using Pydantic settings."""

from __future__ import annotations

import os
from functools import lru_cache
from typing import Annotated
from urllib.parse import urlparse

from fastapi import Depends
from pydantic import field_validator
from pydantic_settings import BaseSettings, NoDecode, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_prefix="PY_FASTAPI_",
        extra="forbid",
    )

    port: int = 8080
    forward_urls: Annotated[list[str], NoDecode] = []
    service_name: str = ""
    log_level: str = "INFO"

    def get_service_name(self) -> str:
        """Return the service name with fallback to OTEL_SERVICE_NAME."""
        if self.service_name:
            return self.service_name
        return os.getenv("OTEL_SERVICE_NAME", "unknown_service")

    @field_validator("log_level")
    @classmethod
    def validate_log_level(cls, v: str) -> str:
        valid_levels = {"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}
        upper = v.upper()
        if upper not in valid_levels:
            raise ValueError(f"log_level must be one of: {', '.join(valid_levels)}")
        return upper

    @field_validator("port")
    @classmethod
    def validate_port(cls, v: int) -> int:
        if not 1 <= v <= 65535:
            raise ValueError("port must be between 1 and 65535")
        return v

    @field_validator("forward_urls", mode="before")
    @classmethod
    def parse_forward_urls(cls, v: str | list[str]) -> list[str]:
        """Parse comma-separated URLs into a list."""
        if isinstance(v, str):
            if not v:
                return []
            return [url.strip() for url in v.split(",") if url.strip()]
        return v

    @field_validator("forward_urls")
    @classmethod
    def validate_forward_urls(cls, v: list[str]) -> list[str]:
        for url in v:
            result = urlparse(url)
            if not result.scheme or not result.netloc:
                raise ValueError(f"invalid forward URL: {url}")
        return v


@lru_cache
def get_settings() -> Settings:
    return Settings()


# Reusable type alias for settings injection
SettingsDep = Annotated[Settings, Depends(get_settings)]
