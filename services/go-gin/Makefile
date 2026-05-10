# Variables
BINARY_NAME=go-gin
VERSION?=dev
IMAGE ?= $(BINARY_NAME)
DIST_DIR=dist
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")
GO=$(shell which go)

# Docker build platform (default: amd64, override with ARCH=arm64)
ARCH ?= amd64
PLATFORM = linux/$(ARCH)

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: Build the application binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "-X main.Version=$(VERSION)" -o $(DIST_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(DIST_DIR)/$(BINARY_NAME)"

## docker-build: Build Docker image (default: amd64, use ARCH=arm64 for ARM)
.PHONY: docker-build
docker-build:
	@echo "Building Docker image $(IMAGE):$(VERSION) for $(PLATFORM)..."
	docker buildx build --platform $(PLATFORM) --build-arg VERSION=$(VERSION) --load -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .
	@echo "Build complete: $(IMAGE):$(VERSION) ($(PLATFORM))"

## docker-publish: Build and push Docker image to registry
.PHONY: docker-publish
docker-publish: docker-build
	@echo "Publishing $(IMAGE):$(VERSION)..."
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest
	@echo "Published $(IMAGE):$(VERSION)"

## run: Run the application with default config
.PHONY: run
run:
	@echo "Running $(BINARY_NAME)..."
	$(GO) run . --port 8080

## test: Run tests with coverage, race detection, and generate HTML report
.PHONY: test
test:
	@echo "Running tests with coverage and race detector..."
	@mkdir -p $(DIST_DIR)
	@$(GO) test \
		-cover \
		-coverprofile=coverage.out \
		-outputdir=$(DIST_DIR) \
		-race \
		-failfast \
		-v \
		-covermode=atomic  ./...
	$(GO) tool cover -html=$(DIST_DIR)/coverage.out -o $(DIST_DIR)/coverage.html
	@echo ""
	@echo "Coverage report generated: $(DIST_DIR)/coverage.html"
	@echo "View with: open $(DIST_DIR)/coverage.html"

## fmt: Format Go code using gofmt
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@gofmt -l -w $(GO_FILES)
	@echo "Code formatted"

## lint: Run golangci-lint for code quality checks (ensures code is formatted first)
.PHONY: lint
lint: fmt
	@echo "Running linters..."
	@golangci-lint run ./...

## check: Alias for lint - validates code quality and formatting
.PHONY: check
check: lint

## security: Run all security checks (lint + govulncheck)
.PHONY: security
security: lint
	@echo "Running govulncheck..."
	@govulncheck ./...

## ci: Run all CI checks (test + security)
.PHONY: ci
ci: test security

## clean: Remove build artifacts
.PHONY: clean
clean:
	@echo "Cleaning $(DIST_DIR)..."
	@rm -rf $(DIST_DIR)
	@echo "Clean complete"
