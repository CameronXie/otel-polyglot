package main

import (
	"testing"

	"go.opentelemetry.io/otel/metric/noop"
)

func TestNewMetrics(t *testing.T) {
	tests := map[string]struct {
		meterName string
		wantError bool
	}{
		"creates metrics with valid meter name": {
			meterName: "test-meter",
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			metrics, err := NewMetrics(tc.meterName)

			if (err != nil) != tc.wantError {
				t.Fatalf("NewMetrics() error = %v, wantError %v", err, tc.wantError)
			}

			if metrics == nil {
				t.Fatal("NewMetrics() returned nil")
			}

			if metrics.Forward == nil {
				t.Error("NewMetrics() Forward is nil")
			}
		})
	}
}

func TestNewForwardMetrics(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		meterName string
	}{
		"creates forward metrics with all instruments": {
			meterName: "test-meter",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			meter := noop.NewMeterProvider().Meter(tc.meterName)

			forward, err := newForwardMetrics(meter)
			if err != nil {
				t.Fatalf("newForwardMetrics() error = %v", err)
			}

			if forward == nil {
				t.Fatal("newForwardMetrics() returned nil")
			}

			if forward.Requests == nil {
				t.Error("Requests counter is nil")
			}

			if forward.Duration == nil {
				t.Error("Duration histogram is nil")
			}
		})
	}
}
