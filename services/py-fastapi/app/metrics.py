"""Custom metric definitions for forward requests."""

from __future__ import annotations

from functools import lru_cache
from typing import TYPE_CHECKING, Final

from opentelemetry import metrics
from opentelemetry.semconv.attributes.server_attributes import SERVER_ADDRESS
from opentelemetry.semconv.attributes.url_attributes import URL_SCHEME

if TYPE_CHECKING:
    from opentelemetry.metrics import Counter, Histogram

_METER_NAME = "py-fastapi"

HISTOGRAM_BUCKETS: Final[tuple[float, ...]] = (
    0.001,
    0.005,
    0.01,
    0.025,
    0.05,
    0.1,
    0.25,
    0.5,
    1.0,
    2.5,
    5.0,
    10.0,
)


@lru_cache(maxsize=1)
def get_forward_metrics() -> ForwardMetrics:
    return ForwardMetrics()


class ForwardMetrics:
    def __init__(self) -> None:
        meter = metrics.get_meter(_METER_NAME)

        self.requests: Counter = meter.create_counter(
            name="forward.requests",
            description="Total outbound forward requests",
            unit="1",
        )

        self.duration: Histogram = meter.create_histogram(
            name="forward.duration",
            description="Outbound forward request duration",
            unit="s",
            explicit_bucket_boundaries_advisory=HISTOGRAM_BUCKETS,
        )

    def record_request(
        self, server_address: str, url_scheme: str, duration: float
    ) -> None:
        attributes = {
            SERVER_ADDRESS: server_address,
            URL_SCHEME: url_scheme,
        }

        self.requests.add(1, attributes=attributes)
        self.duration.record(duration, attributes=attributes)
