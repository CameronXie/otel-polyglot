package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewHandler(t *testing.T) {
	tests := map[string]struct {
		config    *Config
		logger    *slog.Logger
		wantError bool
	}{
		"valid handler creation": {
			config: &Config{
				Port:        "8080",
				ForwardURLs: []string{"http://example.com"},
			},
			logger:    newTestLogger().Logger,
			wantError: false,
		},
		"nil config returns error": {
			config:    nil,
			logger:    newTestLogger().Logger,
			wantError: true,
		},
		"nil logger returns error": {
			config: &Config{
				Port: "8080",
			},
			logger:    nil,
			wantError: true,
		},
		"empty forward urls is valid": {
			config: &Config{
				Port:        "8080",
				ForwardURLs: []string{},
			},
			logger:    newTestLogger().Logger,
			wantError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			handler, err := NewHandler(tc.config, tc.logger)

			if (err != nil) != tc.wantError {
				t.Errorf("NewHandler() error = %v, wantError %v", err, tc.wantError)
				return
			}

			if !tc.wantError {
				if handler == nil {
					t.Fatal("NewHandler() returned nil handler")
				}

				if handler.config != tc.config {
					t.Error("NewHandler() config not set correctly")
				}

				if handler.logger != tc.logger {
					t.Error("NewHandler() logger not set correctly")
				}

				if handler.tracer == nil {
					t.Error("NewHandler() tracer is nil")
				}

				if handler.metrics == nil {
					t.Error("NewHandler() metrics is nil")
				}

				if handler.client == nil {
					t.Error("NewHandler() client is nil")
				}
			}
		})
	}
}

// TestNewHandler_OtelIntegration tests that OTEL components are properly initialized.
func TestNewHandler_OtelIntegration(t *testing.T) {
	t.Run("tracer is initialized", func(t *testing.T) {
		config := &Config{
			Port:        "8080",
			ForwardURLs: []string{"http://example.com"},
		}
		logger := newTestLogger().Logger

		handler, err := NewHandler(config, logger)
		if err != nil {
			t.Fatalf("NewHandler() error = %v", err)
		}

		if handler.tracer == nil {
			t.Error("Expected tracer to be initialized")
		}
	})

	t.Run("metrics are initialized", func(t *testing.T) {
		config := &Config{
			Port:        "8080",
			ForwardURLs: []string{"http://example.com"},
		}
		logger := newTestLogger().Logger

		handler, err := NewHandler(config, logger)
		if err != nil {
			t.Fatalf("NewHandler() error = %v", err)
		}

		if handler.metrics == nil {
			t.Fatal("Expected metrics to be initialized")
		}

		if handler.metrics.Forward == nil {
			t.Error("Expected Forward metrics to be initialized")
		}
	})
}

// TestHealthCheck tests the health check endpoint.
func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := map[string]struct {
		wantStatus int
		wantBody   string
	}{
		"returns healthy status": {
			wantStatus: 200,
			wantBody:   `{"status":"healthy"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/health", http.NoBody)

			HealthCheck(c)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %v, want %v", w.Code, tc.wantStatus)
			}

			if w.Body.String() != tc.wantBody {
				t.Errorf("body = %v, want %v", w.Body.String(), tc.wantBody)
			}
		})
	}
}
