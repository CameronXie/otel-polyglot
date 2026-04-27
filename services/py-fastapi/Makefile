# Variables
PACKAGE_NAME=py-fastapi
VERSION?=0.1.0
DIST_DIR=dist
PYTHON=python
UV=uv

# Docker build platform (default: amd64, override with ARCH=arm64)
ARCH ?= amd64
PLATFORM = linux/$(ARCH)

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## install: Create virtual environment and install dependencies
.PHONY: install
install:
	@echo "Installing dependencies..."
	$(UV) venv
	$(UV) pip install -e . --group dev
	@echo "Dependencies installed in .venv/"

## build: Build wheel package to dist/
.PHONY: build
build:
	@echo "Building $(PACKAGE_NAME)..."
	@mkdir -p $(DIST_DIR)
	$(UV) build --out-dir $(DIST_DIR)
	@echo "Build complete: $(DIST_DIR)/"

## docker-build: Build Docker image (default: amd64, use ARCH=arm64 for ARM)
.PHONY: docker-build
docker-build:
	@echo "Building Docker image $(PACKAGE_NAME):$(VERSION) for $(PLATFORM)..."
	docker buildx build --platform $(PLATFORM) --load -t $(PACKAGE_NAME):$(VERSION) -t $(PACKAGE_NAME):latest .
	@echo "Build complete: $(PACKAGE_NAME):$(VERSION) ($(PLATFORM))"

## run: Run the service locally with hot reload
.PHONY: run
run:
	@echo "Running $(PACKAGE_NAME)..."
	$(UV) run python -m app.main

## test: Run tests with coverage
.PHONY: test
test:
	@echo "Running tests with coverage..."
	@mkdir -p $(DIST_DIR)
	$(UV) run pytest \
		--cov=app \
		--cov-report=term-missing \
		--cov-report=html:$(DIST_DIR)/coverage.html \
		-v \
		tests/
	@echo ""
	@echo "Coverage report generated: $(DIST_DIR)/coverage.html"

## format: Format code with ruff
.PHONY: format
format:
	@echo "Formatting code..."
	$(UV) run ruff format app/ tests/
	@echo "Code formatted"

## format-check: Check code formatting without making changes
.PHONY: format-check
format-check:
	@echo "Checking code formatting..."
	$(UV) run ruff format --check app/ tests/

## lint: Run linters (ruff check + mypy)
.PHONY: lint
lint:
	@echo "Running linters..."
	$(UV) run ruff check app/ tests/
	$(UV) run mypy app/

## security: Run pip-audit for security scanning
.PHONY: security
security:
	@echo "Running security scan..."
	# pip-audit bundles pip 26.0.1 which has CVE-2026-3219; ignore until upstream fixes it
	$(UV) run pip-audit --ignore-vuln CVE-2026-3219

## ci: Run all CI checks (format-check, lint, security, test)
.PHONY: ci
ci: format-check lint security test

## clean: Remove build artifacts
.PHONY: clean
clean:
	@echo "Cleaning $(DIST_DIR)..."
	@rm -rf $(DIST_DIR)
	@rm -rf .ruff_cache
	@rm -rf .mypy_cache
	@echo "Clean complete"
