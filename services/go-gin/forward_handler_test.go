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

func (l *testLogger) assertNotContains(t *testing.T, substr string) {
	t.Helper()
	logOutput := l.buf.String()
	if strings.Contains(logOutput, substr) {
		t.Errorf("log should not contain %q\nactual log:\n%s", substr, logOutput)
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

func TestExecuteForwardRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		serverStatus     int
		serverBody       string
		serverDelay      time.Duration
		client           *http.Client
		want             ForwardResult
		wantErrSubstring string
		wantBodyMaxSize  int
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
		"large body is truncated to max size": {
			serverStatus:    200,
			serverBody:      strings.Repeat("a", 1<<20+1000),
			client:          &http.Client{Timeout: 5 * time.Second},
			want:            ForwardResult{StatusCode: 200},
			wantBodyMaxSize: 1 << 20,
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

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Skip body comparison for truncation tests
			if tc.wantBodyMaxSize == 0 {
				assertForwardResultEqual(t, result, tc.want, server.URL)
			} else {
				// For body size tests, only verify status and size
				if result.StatusCode != tc.want.StatusCode {
					t.Errorf("StatusCode = %v, want %v", result.StatusCode, tc.want.StatusCode)
				}
				if len(result.Body) > tc.wantBodyMaxSize {
					t.Errorf("Body length = %v, want <= %v", len(result.Body), tc.wantBodyMaxSize)
				}
			}

			if tc.serverDelay > 0 && result.Duration < float64(tc.serverDelay.Seconds())*0.8 {
				t.Errorf("Duration = %v, want >= %v (with 20%% tolerance)", result.Duration, tc.serverDelay)
			}
		})
	}
}

func TestForwardSingle(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		rawURL           string
		useServer        bool
		serverStatus     int
		serverDelay      time.Duration
		cancelCtx        bool
		wantErrSubstring string
	}{
		"invalid url format with bad scheme": {
			rawURL:           "://invalid-url",
			wantErrSubstring: "invalid url",
		},
		"invalid url with space": {
			rawURL:           "http://example .com",
			wantErrSubstring: "invalid url",
		},
		"empty url": {
			rawURL:           "",
			wantErrSubstring: "unsupported protocol scheme",
		},
		"missing scheme": {
			rawURL:           "example.com",
			wantErrSubstring: "unsupported protocol scheme",
		},
		"context cancellation": {
			useServer:        true,
			serverStatus:     200,
			serverDelay:      2 * time.Second,
			cancelCtx:        true,
			wantErrSubstring: "context deadline exceeded",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var targetURL string
			var client *http.Client

			if tc.useServer {
				server := newTestServer(t, tc.serverStatus, "ok", tc.serverDelay)
				t.Cleanup(server.Close)
				targetURL = server.URL
				client = &http.Client{Timeout: 5 * time.Second}
			} else {
				targetURL = tc.rawURL
				client = &http.Client{}
			}

			handler, _ := newTestHandler(t, client, nil)

			ctx := context.Background()
			if tc.cancelCtx {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}

			result := handler.forwardSingle(ctx, targetURL)

			if result.Error == "" {
				t.Error("expected result.Error to be set")
			} else if !strings.Contains(result.Error, tc.wantErrSubstring) {
				t.Errorf("result.Error = %q, want contain %q", result.Error, tc.wantErrSubstring)
			}

			if result.URL != targetURL {
				t.Errorf("URL = %q, want %q", result.URL, targetURL)
			}

			// Parse errors don't incur network delay
			if tc.wantErrSubstring != "invalid url" && result.Duration <= 0 {
				t.Errorf("Duration = %v, want > 0", result.Duration)
			}
		})
	}
}

func TestForwardBatch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		servers []struct {
			status int
			body   string
			delay  time.Duration
		}
		cancelCtx bool
		wantCount int
		wantError string
	}{
		"successful batch with multiple servers": {
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{
				{200, "server1", 0},
				{200, "server2", 0},
			},
			wantCount: 2,
		},
		"handles mixed success and failure": {
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{
				{200, "success", 0},
				{500, "error", 0},
			},
			wantCount: 2,
		},
		"handles large batch of URLs": {
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
			wantCount: 10,
		},
		"empty URLs returns empty results": {
			servers: []struct {
				status int
				body   string
				delay  time.Duration
			}{},
			wantCount: 0,
		},
		"context cancellation propagates to workers": {
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
			wantError: "batch processing incomplete",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

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

func TestForwardRequests(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := map[string]struct {
		serverStatus int
		serverBody   string
		forwardURLs  []string
		cancelCtx    bool
		wantStatus   int
		wantRespKey  string
	}{
		"successful batch": {
			serverStatus: 200,
			serverBody:   `{"status":"ok"}`,
			wantStatus:   200,
			wantRespKey:  "results",
		},
		"no urls configured": {
			forwardURLs: []string{},
			wantStatus:  200,
			wantRespKey: "results",
		},
		"upstream error still returns success": {
			serverStatus: 500,
			serverBody:   "internal error",
			wantStatus:   200,
			wantRespKey:  "results",
		},
		"context cancelled returns error": {
			serverStatus: 200,
			serverBody:   "ok",
			cancelCtx:    true,
			wantStatus:   http.StatusInternalServerError,
			wantRespKey:  "error",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testLog := newTestLogger()

			forwardURLs := tc.forwardURLs
			if forwardURLs == nil && tc.serverStatus != 0 {
				server := newTestServer(t, tc.serverStatus, tc.serverBody, 0)
				t.Cleanup(server.Close)
				forwardURLs = []string{server.URL}
			}

			handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, testLog.Logger)
			handler.config.ForwardURLs = forwardURLs

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/forward", http.NoBody)

			if tc.cancelCtx {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				c.Request = c.Request.WithContext(ctx)
			}

			handler.ForwardRequests(c)

			if w.Code != tc.wantStatus {
				t.Errorf("status = %v, want %v", w.Code, tc.wantStatus)
			}

			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse JSON response: %v", err)
			}

			if _, ok := resp[tc.wantRespKey]; !ok {
				t.Errorf("response missing %q key: %v", tc.wantRespKey, resp)
			}
		})
	}
}

func TestForwardRequests_Logging(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := map[string]struct {
		serverStatus       int
		serverBody         string
		forwardURLs        []string
		wantLogContains    []string
		wantLogNotContains []string
	}{
		"logs batch lifecycle on success": {
			serverStatus:    200,
			serverBody:      "ok",
			wantLogContains: []string{"Starting forward batch", "Forward batch completed"},
		},
		"logs upstream errors": {
			serverStatus:    500,
			serverBody:      "internal error",
			wantLogContains: []string{"Upstream returned error status"},
		},
		"logs batch error on cancellation": {
			serverStatus:    200,
			serverBody:      "ok",
			forwardURLs:     []string{"http://will-be-cancelled"},
			wantLogContains: []string{"Batch processing failed"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testLog := newTestLogger()

			forwardURLs := tc.forwardURLs
			cancelCtx := false
			if forwardURLs != nil {
				cancelCtx = true
			} else {
				server := newTestServer(t, tc.serverStatus, tc.serverBody, 0)
				t.Cleanup(server.Close)
				forwardURLs = []string{server.URL}
			}

			handler, _ := newTestHandler(t, &http.Client{Timeout: 5 * time.Second}, testLog.Logger)
			handler.config.ForwardURLs = forwardURLs

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/forward", http.NoBody)

			if cancelCtx {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				c.Request = c.Request.WithContext(ctx)
			}

			handler.ForwardRequests(c)

			for _, want := range tc.wantLogContains {
				testLog.assertContains(t, want)
			}
			for _, notWant := range tc.wantLogNotContains {
				testLog.assertNotContains(t, notWant)
			}
		})
	}
}
