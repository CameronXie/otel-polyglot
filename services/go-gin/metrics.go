package main

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// ForwardMetrics contains metrics for forward requests.
type ForwardMetrics struct {
	Requests metric.Int64Counter
	Duration metric.Float64Histogram
}

// Metrics contains all application metrics.
type Metrics struct {
	Forward *ForwardMetrics
}

// NewMetrics creates a Metrics instance with forward request metrics.
func NewMetrics(meterName string) (*Metrics, error) {
	meter := otel.Meter(meterName)

	forward, err := newForwardMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("forward metrics: %w", err)
	}

	return &Metrics{
		Forward: forward,
	}, nil
}

func newForwardMetrics(meter metric.Meter) (*ForwardMetrics, error) {
	requests, err := meter.Int64Counter(
		"forward.requests",
		metric.WithDescription("Total outbound forward requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, fmt.Errorf("create forward.requests: %w", err)
	}

	duration, err := meter.Float64Histogram(
		"forward.duration",
		metric.WithDescription("Outbound forward request duration"),
		metric.WithUnit("s"),
		//nolint:mnd // Standard histogram bucket boundaries for request duration (seconds)
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create forward.duration: %w", err)
	}

	return &ForwardMetrics{
		Requests: requests,
		Duration: duration,
	}, nil
}
