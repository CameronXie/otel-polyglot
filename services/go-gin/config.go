package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds application configuration.
type Config struct {
	Port        string
	ForwardURLs []string
	ServiceName string
	LogLevel    slog.Level
}

// LoadConfig loads configuration from flags and environment variables.
func LoadConfig(args []string) (*Config, error) {
	flagSet := pflag.NewFlagSet("go-gin", pflag.ContinueOnError)
	flagSet.String("port", "8080", "Server port")
	flagSet.StringSlice("forward-urls", []string{}, "List of URLs to forward requests (GET is assumed)")
	flagSet.String("service-name", "", "Service name (defaults to OTEL_SERVICE_NAME env var)")
	flagSet.String("log-level", "info", "Log level (debug, info, warn, error)")

	if err := flagSet.Parse(args); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	v := viper.New()
	if err := v.BindPFlags(flagSet); err != nil {
		return nil, fmt.Errorf("failed to bind flags: %w", err)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	v.SetEnvPrefix("GO_GIN")

	logLevel, err := parseLogLevel(v.GetString("log-level"))
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:        v.GetString("port"),
		ForwardURLs: parseStringSlice(v.GetStringSlice("forward-urls")),
		ServiceName: v.GetString("service-name"),
		LogLevel:    logLevel,
	}, nil
}

// parseStringSlice works around viper#380: GetStringSlice parses CLI flags
// as comma-separated but env vars as space-separated.
func parseStringSlice(slice []string) []string {
	if len(slice) == 1 && strings.Contains(slice[0], ",") {
		return strings.Split(slice[0], ",")
	}
	return slice
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) (slog.Level, error) {
	validLevels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	if l, ok := validLevels[strings.ToLower(level)]; ok {
		return l, nil
	}
	validKeys := make([]string, 0, len(validLevels))
	for k := range validLevels {
		validKeys = append(validKeys, k)
	}
	return slog.LevelInfo, fmt.Errorf("log_level must be one of: %s", strings.Join(validKeys, ", "))
}

// GetServiceName returns the service name, falling back to OTEL_SERVICE_NAME env var or "unknown_service".
func (c *Config) GetServiceName() string {
	if c.ServiceName != "" {
		return c.ServiceName
	}

	if env := os.Getenv("OTEL_SERVICE_NAME"); env != "" {
		return env
	}

	return "unknown_service"
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("port is required")
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be a valid number between 1 and 65535")
	}
	for _, u := range c.ForwardURLs {
		if _, err := url.Parse(u); err != nil {
			return fmt.Errorf("invalid forward URL %q: %w", u, err)
		}
	}
	return nil
}
