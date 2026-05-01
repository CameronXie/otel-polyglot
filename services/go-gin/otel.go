package main

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// InitOtel sets up OpenTelemetry providers and returns a shutdown function.
func InitOtel(ctx context.Context, config *Config) (shutdown func(context.Context) error, err error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	res, err := newResource(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tracerProvider, err := newTracerProvider(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("create tracer provider: %w", err)
	}

	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("create meter provider: %w", err)
	}

	loggerProvider, err := newLoggerProvider(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("create logger provider: %w", err)
	}

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown = func(ctx context.Context) error {
		var errs error
		if err := tracerProvider.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("meter shutdown: %w", err))
		}
		if err := loggerProvider.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("logger shutdown: %w", err))
		}
		return errs
	}

	return shutdown, nil
}

func newResource(ctx context.Context, config *Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(config.GetServiceName()),
			semconv.ServiceVersion(Version),
		),
	)
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("create span exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	reader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("create metric reader: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	), nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exporter, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("create log exporter: %w", err)
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	), nil
}
