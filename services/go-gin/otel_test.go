package main

import (
	"context"
	"testing"
	"time"

	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
)

func TestNewResource(t *testing.T) {
	tests := map[string]struct {
		serviceName    string
		wantError      bool
		wantServiceSvc string
	}{
		"creates resource with valid config": {
			serviceName:    "test-service",
			wantError:      false,
			wantServiceSvc: "test-service",
		},
		"creates resource with empty service name": {
			serviceName:    "",
			wantError:      false,
			wantServiceSvc: "unknown_service",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := &Config{
				ServiceName: tc.serviceName,
				Port:        "8080",
			}

			res, err := newResource(context.Background(), config)

			if (err != nil) != tc.wantError {
				t.Errorf("newResource() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if res == nil {
				t.Fatal("newResource() returned nil resource")
			}

			// Verify service name attribute
			attrValue, exists := res.Set().Value(semconv.ServiceNameKey)
			if !exists {
				t.Error("service name attribute not found in resource")
			}
			if attrValue.AsString() != tc.wantServiceSvc {
				t.Errorf("service name = %v, want %v", attrValue.AsString(), tc.wantServiceSvc)
			}
		})
	}
}

func TestInitOtel(t *testing.T) {
	tests := map[string]struct {
		config    *Config
		wantError bool
		protocol  string
	}{
		"initializes all providers": {
			config: &Config{
				ServiceName: "test-service",
				Port:        "8080",
			},
			wantError: false,
			protocol:  "grpc",
		},
		"initializes with empty service name": {
			config: &Config{
				ServiceName: "",
				Port:        "8080",
			},
			wantError: false,
			protocol:  "grpc",
		},
		"nil config returns error": {
			config:    nil,
			wantError: true,
		},
		"uses http/protobuf protocol": {
			config: &Config{
				ServiceName: "test-service",
				Port:        "8080",
			},
			wantError: false,
			protocol:  "http/protobuf",
		},
		"default protocol when env unset": {
			config: &Config{
				ServiceName: "test-service",
				Port:        "8080",
			},
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.protocol != "" {
				t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", tc.protocol)
			}

			shutdown, err := InitOtel(context.Background(), tc.config)

			if (err != nil) != tc.wantError {
				t.Errorf("InitOtel() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if !tc.wantError {
				if shutdown == nil {
					t.Error("InitOtel() returned nil shutdown function")
					return
				}

				// Register cleanup to shutdown providers
				t.Cleanup(func() {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = shutdown(shutdownCtx) // errors OK (no collector), but shouldn't panic
				})
			}
		})
	}
}
