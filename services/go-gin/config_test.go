package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	defaultPort = "8080"
	defaultEnv  = "development"
)

func TestConfig_GetServiceName(t *testing.T) {
	tests := map[string]struct {
		config          *Config
		envVar          string
		wantServiceName string
	}{
		"config service name takes precedence": {
			config: &Config{
				ServiceName: "my-service",
			},
			envVar:          "env-service",
			wantServiceName: "my-service",
		},
		"env var used when config empty": {
			config: &Config{
				ServiceName: "",
			},
			envVar:          "env-service",
			wantServiceName: "env-service",
		},
		"fallback to unknown_service": {
			config: &Config{
				ServiceName: "",
			},
			envVar:          "",
			wantServiceName: "unknown_service",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.envVar != "" {
				t.Setenv("OTEL_SERVICE_NAME", tc.envVar)
			}

			got := tc.config.GetServiceName()
			if got != tc.wantServiceName {
				t.Errorf("GetServiceName() = %q, want %q", got, tc.wantServiceName)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := map[string]struct {
		args           []string
		envVars        map[string]string
		wantPort       string
		wantService    string
		wantEnv        string
		wantForwardURL []string
		wantErr        bool
		wantErrContain string
	}{
		"default values": {
			args:     []string{},
			wantPort: defaultPort,
			wantEnv:  defaultEnv,
		},
		"custom port via flag": {
			args:     []string{"--port=9090"},
			wantPort: "9090",
			wantEnv:  defaultEnv,
		},
		"custom port via env": {
			args:     []string{},
			envVars:  map[string]string{"GO_GIN_PORT": "3000"},
			wantPort: "3000",
			wantEnv:  defaultEnv,
		},
		"service name via flag": {
			args:        []string{"--service-name=my-service"},
			wantPort:    defaultPort,
			wantService: "my-service",
			wantEnv:     defaultEnv,
		},
		"environment via flag": {
			args:     []string{"--environment=production"},
			wantPort: defaultPort,
			wantEnv:  "production",
		},
		"forward URLs via flag": {
			args:           []string{"--forward-urls=http://a.com,http://b.com"},
			wantPort:       defaultPort,
			wantEnv:        defaultEnv,
			wantForwardURL: []string{"http://a.com", "http://b.com"},
		},
		"all values via env": {
			args:        []string{},
			envVars:     map[string]string{"GO_GIN_PORT": "5000", "GO_GIN_SERVICE_NAME": "env-service", "GO_GIN_ENVIRONMENT": "staging"},
			wantPort:    "5000",
			wantService: "env-service",
			wantEnv:     "staging",
		},
		"flag takes precedence over env": {
			args:     []string{"--port=9090"},
			envVars:  map[string]string{"GO_GIN_PORT": "3000"},
			wantPort: "9090",
			wantEnv:  defaultEnv,
		},
		"unknown flag returns error": {
			args:           []string{"--unknown-flag"},
			wantErr:        true,
			wantErrContain: "unknown-flag",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			config, err := LoadConfig(tc.args)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("LoadConfig() expected error containing %q, got nil", tc.wantErrContain)
				}
				if !strings.Contains(err.Error(), tc.wantErrContain) {
					t.Errorf("LoadConfig() error = %q, want containing %q", err.Error(), tc.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}

			if config.Port != tc.wantPort {
				t.Errorf("Port = %q, want %q", config.Port, tc.wantPort)
			}
			if tc.wantService != "" && config.ServiceName != tc.wantService {
				t.Errorf("ServiceName = %q, want %q", config.ServiceName, tc.wantService)
			}
			if config.Environment != tc.wantEnv {
				t.Errorf("Environment = %q, want %q", config.Environment, tc.wantEnv)
			}
			if tc.wantForwardURL != nil {
				if diff := cmp.Diff(tc.wantForwardURL, config.ForwardURLs); diff != "" {
					t.Errorf("ForwardURLs mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		config       *Config
		wantError    bool
		wantErrorMsg string
	}{
		"valid config": {
			config: &Config{
				Port: defaultPort,
			},
			wantError: false,
		},
		"empty port": {
			config: &Config{
				Port: "",
			},
			wantError:    true,
			wantErrorMsg: "port is required",
		},
		"non-numeric port": {
			config: &Config{
				Port: "abc",
			},
			wantError:    true,
			wantErrorMsg: "port must be a valid number between 1 and 65535",
		},
		"port too low": {
			config: &Config{
				Port: "0",
			},
			wantError:    true,
			wantErrorMsg: "port must be a valid number between 1 and 65535",
		},
		"port too high": {
			config: &Config{
				Port: "65536",
			},
			wantError:    true,
			wantErrorMsg: "port must be a valid number between 1 and 65535",
		},
		"valid forward URLs": {
			config: &Config{
				Port:        defaultPort,
				ForwardURLs: []string{"http://example.com", "https://api.example.com/path"},
			},
			wantError: false,
		},
		"empty forward URLs is valid": {
			config: &Config{
				Port:        defaultPort,
				ForwardURLs: []string{},
			},
			wantError: false,
		},
		"invalid forward URL with space": {
			config: &Config{
				Port:        defaultPort,
				ForwardURLs: []string{"http://example .com"},
			},
			wantError:    true,
			wantErrorMsg: `invalid forward URL "http://example .com"`,
		},
		"invalid forward URL with bad scheme": {
			config: &Config{
				Port:        defaultPort,
				ForwardURLs: []string{"://invalid-url"},
			},
			wantError:    true,
			wantErrorMsg: `invalid forward URL "://invalid-url"`,
		},
		"multiple URLs with one invalid": {
			config: &Config{
				Port:        defaultPort,
				ForwardURLs: []string{"http://valid.com", "://invalid"},
			},
			wantError:    true,
			wantErrorMsg: `invalid forward URL "://invalid"`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantError {
				if err == nil {
					t.Fatalf("Validate() expected error containing %q, got nil", tc.wantErrorMsg)
				}
				if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tc.wantErrorMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}
