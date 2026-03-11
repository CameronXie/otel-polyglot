package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "github.com/CameronXie/services/go-gin"
	defaultHTTPTimeout  = 2 * time.Second
)

// Handler handles HTTP requests with OTel instrumentation.
type Handler struct {
	config  *Config
	client  *http.Client
	tracer  trace.Tracer
	metrics *Metrics
	logger  *slog.Logger
}

// NewHandler creates a Handler with the given config and logger.
func NewHandler(config *Config, logger *slog.Logger) (*Handler, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}

	tracer := otel.Tracer(instrumentationName)
	metrics, err := NewMetrics(instrumentationName)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   defaultHTTPTimeout,
	}

	return &Handler{
		config:  config,
		logger:  logger,
		client:  client,
		tracer:  tracer,
		metrics: metrics,
	}, nil
}
