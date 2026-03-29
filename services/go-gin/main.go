package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

const (
	shutdownTimeout    = 10 * time.Second
	serverReadTimeout  = 15 * time.Second
	serverWriteTimeout = 15 * time.Second
	serverIdleTimeout  = 60 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("Service failed", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Service exited successfully")
}

func run() error {
	ctx := context.Background()

	config, err := LoadConfig(os.Args[1:])
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	slog.SetLogLoggerLevel(config.LogLevel)
	slog.Info("Starting service", slog.String("port", config.Port)) //nolint:gosec // G706

	shutdown, err := InitOtel(ctx, config)
	if err != nil {
		return fmt.Errorf("init otel: %w", err)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			slog.Error("Error shutting down OpenTelemetry", slog.Any("error", err))
		}
	}()

	return runServer(ctx, config)
}

func runServer(ctx context.Context, config *Config) error {
	logger := otelslog.NewLogger(instrumentationName)

	handler, err := NewHandler(config, logger)
	if err != nil {
		return fmt.Errorf("create handler: %w", err)
	}

	router := gin.Default()
	router.Use(otelgin.Middleware(config.GetServiceName()))
	router.GET("/health", HealthCheck)
	router.GET("/forward", handler.ForwardRequests)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", config.Port),
		Handler:      router,
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		IdleTimeout:  serverIdleTimeout,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
	}

	errChan := make(chan error, 1)
	go func() {
		slog.Info("Server listening", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("server failed: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		slog.Info("Shutdown signal received")
	case err := <-errChan:
		return err
	}

	slog.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	slog.Info("Server shutdown complete")
	return nil
}
