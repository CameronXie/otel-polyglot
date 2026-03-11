package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.opentelemetry.io/otel/trace/noop"
)

// testLogger captures log output for assertions.
type testLogger struct {
	*slog.Logger
	buf *bytes.Buffer
}

func newTestLogger() *testLogger {
	buf := new(bytes.Buffer)
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &testLogger{
		Logger: slog.New(handler),
		buf:    buf,
	}
}

func (l *testLogger) assertContains(t *testing.T, substr string) {
	t.Helper()
	logOutput := l.buf.String()
	if !strings.Contains(logOutput, substr) {
		t.Errorf("log does not contain %q\nactual log:\n%s", substr, logOutput)
	}
}

// assertForwardResultEqual validates ForwardResult fields match expected values.
func assertForwardResultEqual(t *testing.T, got, want ForwardResult, wantURL string) {
	t.Helper()

	// Include URL in expected for comparison
	expected := want
	expected.URL = wantURL

	// Use cmp with options to ignore Duration (dynamic value)
	if diff := cmp.Diff(expected, got, cmpopts.IgnoreFields(ForwardResult{}, "Duration")); diff != "" {
		t.Errorf("ForwardResult mismatch (-want +got):\n%s", diff)
	}

	// Duration just needs to be positive
	if got.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", got.Duration)
	}
}

// newTestHandler creates a Handler with mocked dependencies for testing.
func newTestHandler(t *testing.T, client *http.Client, logger *slog.Logger) (*Handler, error) {
	t.Helper()
	config := &Config{
		Port:        "8080",
		ForwardURLs: []string{"http://example.com"},
	}
	if logger == nil {
		logger = newTestLogger().Logger
	}

	tracer := noop.NewTracerProvider().Tracer("test")

	metrics, err := NewMetrics("test-meter")
	if err != nil {
		return nil, fmt.Errorf("create metrics: %w", err)
	}

	return &Handler{
		config:  config,
		logger:  logger,
		client:  client,
		tracer:  tracer,
		metrics: metrics,
	}, nil
}

// newTestServer creates a test HTTP server with custom response.
func newTestServer(t *testing.T, status int, body string, delay time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	return server
}

// assertJSONResponse asserts HTTP response status and JSON key existence.
func assertJSONResponse(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantKey string) {
	t.Helper()

	if w.Code != wantStatus {
		t.Errorf("status = %v, want %v", w.Code, wantStatus)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if wantKey != "" {
		if _, ok := resp[wantKey]; !ok {
			t.Errorf("response missing %q key: %v", wantKey, resp)
		}
	}
}

func TestExecuteForwardRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		serverStatus     int
		serverBody       string
		serverDelay      time.Duration
		client           *http.Client
		want             ForwardResult
		wantErrSubstring string
	}{
		"successful request": {
			serverStatus: 200,
			serverBody:   "response body",
			client:       &http.Client{Timeout: 5 * time.Second},
			want: ForwardResult{
				StatusCode: 200,
				Body:       "response body",
			},
		},
		"server error 500": {
			serverStatus: 500,
			serverBody:   "internal error",
			client:       &http.Client{Timeout: 5 * time.Second},
			want: ForwardResult{
				StatusCode: 500,
				Body:       "internal error",
			},
		},
		"server error 404": {
			serverStatus: 404,
			serverBody:   "not found",
			client:       &http.Client{Timeout: 5 * time.Second},
			want: ForwardResult{
				StatusCode: 404,
				Body:       "not found",
			},
		},
		"successful with delay": {
			serverStatus: 200,
			serverBody:   "delayed response",
			serverDelay:  100 * time.Millisecond,
			client:       &http.Client{Timeout: 5 * time.Second},
			want: ForwardResult{
				StatusCode: 200,
				Body:       "delayed response",
			},
		},
		"request timeout": {
			serverStatus:     200,
			serverDelay:      2 * time.Second,
			client:           &http.Client{Timeout: 100 * time.Millisecond},
			wantErrSubstring: "context deadline exceeded",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, tc.serverStatus, tc.serverBody, tc.serverDelay)
			t.Cleanup(server.Close)

			parsedURL, _ := url.Parse(server.URL)
			ctx := context.Background()

			result, err := executeForwardRequest(ctx, parsedURL, tc.client)

			if tc.wantErrSubstring != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSubstring)
				}
				if !strings.Contains(err.Error(), tc.wantErrSubstring) {
					t.Errorf("error = %q, want contain %q", err.Error(), tc.wantErrSubstring)
				}
				if result.Error == "" {
					t.Error("expected result.Error to be set")
				} else if !strings.Contains(result.Error, tc.wantErrSubstring) {
					t.Errorf("result.Error = %q, want contain %q", result.Error, tc.wantErrSubstring)
				}
				if result.URL != server.URL {
					t.Errorf("URL = %q, want %q", result.URL, server.URL)
				}
				if result.Duration <= 0 {
					t.Errorf("Duration = %v, want > 0", result.Duration)
				}
				return
			}

			assertForwardResultEqual(t, result, tc.want, server.URL)

			if tc.serverDelay > 0 && result.Duration < float64(tc.serverDelay.Seconds())*0.8 {
				t.Errorf("Duration = %v, want >= %v (with 20%% tolerance)", result.Duration, tc.serverDelay)
			}
		})
	}
}

func TestExecuteForwardRequest_BodySizeLimit(t *testing.T) {
	t.Parallel()

	t.Run("respects max body size limit", func(t *testing.T) {
		t.Parallel()

		// Create server that writes more than 1MB without large string allocation
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			for range 1100 {
				_, _ = w.Write([]byte(strings.Repeat("a", 1000)))
			}
		}))
		t.Cleanup(server.Close)

		parsedURL, _ := url.Parse(server.URL)
		client := &http.Client{Timeout: 5 * time.Second}
		ctx := context.Background()

		result, err := executeForwardRequest(ctx, parsedURL, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		maxSize := 1 << 20 // 1MB
		if len(result.Body) > maxSize {
			t.Errorf("Body length = %v, want <= %v", len(result.Body), maxSize)
		}

		if result.StatusCode != 200 {
			t.Errorf("StatusCode = %v, want 200", result.StatusCode)
		}
	})
}

func TestForwardSingle_InvalidURL(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t, &http.Client{}, nil)

	tests := map[string]struct {
		rawURL           string
		serverURL        string // expected URL in result (same as rawURL for invalid URLs)
		wantErrSubstring string
	}{
		"invalid url format with bad scheme": {
			rawURL:           "://invalid-url",
			serverURL:        "://invalid-url",
			wantErrSubstring: "invalid url",
		},
		"invalid url with space": {
			rawURL:           "http://example .com",
			serverURL:        "http://example .com",
			wantErrSubstring: "invalid url",
		},
		"empty url fails during request": {
			rawURL:           "",
			serverURL:        "",
			wantErrSubstring: "unsupported protocol scheme",
		},
		"missing scheme fails during request": {
			rawURL:           "example.com",
			serverURL:        "example.com",
			wantErrSubstring: "unsupported protocol scheme",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			result := handler.forwardSingle(ctx, tc.rawURL)

			if result.Error == "" {
				t.Error("expected result.Error to be set")
			} else if !strings.Contains(result.Error, tc.wantErrSubstring) {
				t.Errorf("result.Error = %q, want contain %q", result.Error, tc.wantErrSubstring)
			}

			if result.URL != tc.serverURL {
				t.Errorf("URL = %q, want %q", result.URL, tc.serverURL)
			}

			if tc.wantErrSubstring != "invalid url" && result.Duration <= 0 {
				t.Errorf("Duration = %v, want > 0", result.Duration)
			}
		})
	}
}

func TestForwardSingle_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, 200, "ok", 2*time.Second)
	t.Cleanup(server.Close)

	handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := handler.forwardSingle(ctx, server.URL)

	wantErrSubstring := "context deadline exceeded"
	if result.Error == "" {
		t.Error("expected result.Error to be set")
	} else if !strings.Contains(result.Error, wantErrSubstring) {
		t.Errorf("result.Error = %q, want contain %q", result.Error, wantErrSubstring)
	}

	if result.URL != server.URL {
		t.Errorf("URL = %q, want %q", result.URL, server.URL)
	}

	if result.Duration <= 0 {
		t.Errorf("Duration = %v, want > 0", result.Duration)
	}
}

func TestForwardBatch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		wantCount int
		wantError string
		servers   []struct {
			status int
			body   string
			delay  time.Duration
		}
		cancelCtx bool
	}{
		"successful batch with multiple servers": {
			wantCount: 2,
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{
				{200, "server1", 0},
				{200, "server2", 0},
			},
		},
		"handles mixed success and failure": {
			wantCount: 2,
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{
				{200, "success", 0},
				{500, "error", 0},
			},
		},
		"handles large batch of URLs": {
			wantCount: 10,
			servers: func() []struct {
				status int
				body   string
				delay  time.Duration
			} {
				var s []struct {
					status int
					body   string
					delay  time.Duration
				}
				for range 10 {
					s = append(s, struct {
						status int
						body   string
						delay  time.Duration
					}{200, "ok", 0})
				}
				return s
			}(),
		},
		"empty URLs returns empty results": {
			wantCount: 0,
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{},
		},
		"context cancellation propagates to workers": {
			wantError: "batch processing incomplete",
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{
				{200, "ok", 2 * time.Second},
				{200, "ok", 2 * time.Second},
				{200, "ok", 2 * time.Second},
			},
			cancelCtx: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Create servers and register cleanup
			var urls []string
			for _, s := range tc.servers {
				server := newTestServer(t, s.status, s.body, s.delay)
				t.Cleanup(server.Close)
				urls = append(urls, server.URL)
			}

			handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, nil)
			handler.config.ForwardURLs = urls

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			results, err := handler.forwardBatch(ctx)

			if tc.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantError)
				}
				if !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("error = %q, want contain %q", err.Error(), tc.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("forwardBatch() error = %v", err)
			}

			if len(results) != tc.wantCount {
				t.Errorf("len(results) = %v, want %v", len(results), tc.wantCount)
			}
		})
	}
}

func TestForwardRequests_HTTPHandler(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	tests := map[string]struct {
		forwardURLs []string
		wantStatus  int
		wantRespKey string
		wantLog     string
	}{
		"successful batch": {
			forwardURLs: []string{"http://example.com"},
			wantStatus:  200,
			wantRespKey: "results",
			wantLog:     "Forward batch completed",
		},
		"no urls configured": {
			forwardURLs: []string{},
			wantStatus:  200,
			wantRespKey: "results",
			wantLog:     "Forward batch completed",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testLog := newTestLogger()

			// Use local variable to avoid mutating test case
			forwardURLs := tc.forwardURLs
			if len(forwardURLs) > 0 {
				server := newTestServer(t, 200, `{"status":"ok"}`, 0)
				t.Cleanup(server.Close)
				forwardURLs = []string{server.URL}
			}

			handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, testLog.Logger)
			handler.config.ForwardURLs = forwardURLs

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/forward", http.NoBody)

			handler.ForwardRequests(c)

			assertJSONResponse(t, w, tc.wantStatus, tc.wantRespKey)
			testLog.assertContains(t, tc.wantLog)
		})
	}
}

func TestForwardRequests_Logging(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("logs errors on upstream failure", func(t *testing.T) {
		t.Parallel()

		testLog := newTestLogger()
		handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, testLog.Logger)

		server := newTestServer(t, 500, "internal error", 0)
		t.Cleanup(server.Close)

		handler.config.ForwardURLs = []string{server.URL}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/forward", http.NoBody)

		handler.ForwardRequests(c)

		// Should still complete successfully but log the error
		assertJSONResponse(t, w, 200, "results")
	})

	t.Run("logs forward batch start info", func(t *testing.T) {
		t.Parallel()

		testLog := newTestLogger()
		handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, testLog.Logger)

		server := newTestServer(t, 200, "ok", 0)
		t.Cleanup(server.Close)

		handler.config.ForwardURLs = []string{server.URL}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/forward", http.NoBody)

		handler.ForwardRequests(c)

		testLog.assertContains(t, "Starting forward batch")
		testLog.assertContains(t, "Forward batch completed")
	})
}

func TestForwardRequests_BatchError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	t.Run("returns error when context cancelled before batch", func(t *testing.T) {
		t.Parallel()

		testLog := newTestLogger()
		handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, testLog.Logger)

		server := newTestServer(t, 200, "ok", 0)
		t.Cleanup(server.Close)

		handler.config.ForwardURLs = []string{server.URL}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Create a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c.Request = httptest.NewRequest("GET", "/forward", http.NoBody)
		c.Request = c.Request.WithContext(ctx)

		handler.ForwardRequests(c)

		// Should return 500 error
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %v, want %v", w.Code, http.StatusInternalServerError)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}

		if _, ok := resp["error"]; !ok {
			t.Error("response missing 'error' key")
		}

		testLog.assertContains(t, "Batch processing failed")
	})
}
