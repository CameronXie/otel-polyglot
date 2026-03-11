package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const maxResponseBodySize = 1 << 20 // 1 MB

type ForwardResult struct {
	URL        string  `json:"url"`
	StatusCode int     `json:"status_code,omitempty"`
	Body       string  `json:"body,omitempty"`
	Error      string  `json:"error,omitempty"`
	Duration   float64 `json:"duration_seconds,omitempty"`
}

func (h *Handler) ForwardRequests(c *gin.Context) {
	ctx := c.Request.Context()
	bag := baggage.FromContext(ctx)

	// Add baggage to span attributes if present
	spanAttrs := []attribute.KeyValue{
		attribute.StringSlice("forward.urls", h.config.ForwardURLs),
	}
	if bag.Len() > 0 {
		spanAttrs = append(spanAttrs, attribute.String("baggage", bag.String()))
	}

	ctx, span := h.tracer.Start(ctx, "forward.batch", trace.WithAttributes(spanAttrs...))
	defer span.End()

	h.logger.InfoContext(ctx, "Starting forward batch",
		slog.Int("url.count", len(h.config.ForwardURLs)),
		slog.String("baggage", bag.String()),
	)

	results, err := h.forwardBatch(ctx)
	if err != nil {
		h.logger.ErrorContext(ctx, "Batch processing failed", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "batch failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "batch processing failed"})
		return
	}

	span.SetAttributes(attribute.Int("forward.batch_size", len(h.config.ForwardURLs)))
	span.SetStatus(codes.Ok, "batch completed")
	h.logger.InfoContext(ctx, "Forward batch completed", slog.Int("results.count", len(results)))

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *Handler) forwardBatch(ctx context.Context) ([]ForwardResult, error) {
	urlCount := len(h.config.ForwardURLs)
	resultsChan := make(chan ForwardResult, urlCount)

	g, ctx := errgroup.WithContext(ctx)

	for _, rawURL := range h.config.ForwardURLs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				result := h.forwardSingle(ctx, rawURL)
				resultsChan <- result
				return nil
			}
		})
	}

	err := g.Wait()
	close(resultsChan)

	if err != nil {
		return nil, fmt.Errorf("batch processing incomplete: %w", err)
	}

	results := make([]ForwardResult, 0, urlCount)
	for r := range resultsChan {
		results = append(results, r)
	}

	return results, nil
}

func (h *Handler) forwardSingle(ctx context.Context, rawURL string) ForwardResult {
	url, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ForwardResult{
			URL:   rawURL,
			Error: "invalid url",
		}
	}

	ctx, span := h.tracer.Start(ctx, "forward.request",
		trace.WithAttributes(
			semconv.URLFull(rawURL),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	logger := h.logger.With(slog.String("url.full", url.String()))

	result, err := executeForwardRequest(ctx, url, h.client)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to execute request", slog.Any("error", err))
		span.RecordError(err)
		span.SetStatus(codes.Error, "request failed")
		return result
	}

	recordMetrics(ctx, h.metrics, url, result)

	span.SetAttributes(
		semconv.HTTPResponseStatusCode(result.StatusCode),
		semconv.HTTPRequestMethodGet,
	)
	if result.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, "upstream error")
		logger.WarnContext(ctx, "Upstream returned error status",
			slog.Int("http.response.status_code", result.StatusCode),
		)

		return result
	}

	span.SetStatus(codes.Ok, "request completed")
	logger.InfoContext(ctx, "Request completed successfully")

	return result
}

func executeForwardRequest(ctx context.Context, url *neturl.URL, client *http.Client) (ForwardResult, error) {
	startTime := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), http.NoBody)
	if err != nil {
		return ForwardResult{
			URL:      url.String(),
			Error:    "failed to create request",
			Duration: time.Since(startTime).Seconds(),
		}, fmt.Errorf("create request: %w", err)
	}

	//nolint:gosec // URLs are admin-configured via FORWARD_URLS, not user input
	resp, err := client.Do(req)
	if err != nil {
		return ForwardResult{
			URL:      url.String(),
			Error:    err.Error(),
			Duration: time.Since(startTime).Seconds(),
		}, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return ForwardResult{
			URL:        url.String(),
			StatusCode: resp.StatusCode,
			Error:      "failed to read response body",
			Duration:   time.Since(startTime).Seconds(),
		}, fmt.Errorf("read response body: %w", err)
	}

	return ForwardResult{
		URL:        url.String(),
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Duration:   time.Since(startTime).Seconds(),
	}, nil
}

func recordMetrics(ctx context.Context, metrics *Metrics, url *neturl.URL, result ForwardResult) {
	opt := metric.WithAttributes(
		semconv.ServerAddress(url.Host),
		semconv.URLScheme(url.Scheme),
	)

	metrics.Forward.Requests.Add(ctx, 1, opt)
	metrics.Forward.Duration.Record(ctx, result.Duration, opt)
}
