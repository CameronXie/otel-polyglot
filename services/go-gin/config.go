package main

import (
	"fmt"
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
	Environment string
}

// LoadConfig loads configuration from flags and environment variables.
func LoadConfig(args []string) (*Config, error) {
	flagSet := pflag.NewFlagSet("go-gin", pflag.ContinueOnError)
	flagSet.String("port", "8080", "Server port")
	flagSet.StringSlice("forward-urls", []string{}, "List of URLs to forward requests (GET is assumed)")
	flagSet.String("service-name", "", "Service name (defaults to OTEL_SERVICE_NAME env var)")
	flagSet.String("environment", "development", "Environment name (e.g., development, production)")

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

	return &Config{
		Port:        v.GetString("port"),
		ForwardURLs: v.GetStringSlice("forward-urls"),
		ServiceName: v.GetString("service-name"),
		Environment: v.GetString("environment"),
	}, nil
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
