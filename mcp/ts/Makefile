# Variables
PACKAGE_NAME=ts-mcp
VERSION?=0.1.0
PNPM=pnpm

# Docker build platform (default: amd64, override with ARCH=arm64)
ARCH ?= amd64
PLATFORM = linux/$(ARCH)

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## install: Install dependencies
.PHONY: install
install:
	@echo "Installing dependencies..."
	$(PNPM) install
	@echo "Dependencies installed"

## build: Compile TypeScript to dist/
.PHONY: build
build:
	@echo "Building $(PACKAGE_NAME)..."
	$(PNPM) run build
	@echo "Build complete: dist/"

## docker-build: Build Docker image (default: amd64, use ARCH=arm64 for ARM)
.PHONY: docker-build
docker-build:
	@echo "Building Docker image $(PACKAGE_NAME):$(VERSION) for $(PLATFORM)..."
	docker buildx build --platform $(PLATFORM) --load -t $(PACKAGE_NAME):$(VERSION) -t $(PACKAGE_NAME):latest .
	@echo "Build complete: $(PACKAGE_NAME):$(VERSION) ($(PLATFORM))"

## run: Run the MCP server
.PHONY: run
run:
	@echo "Running $(PACKAGE_NAME)..."
	$(PNPM) run start

## test: Run tests with coverage
.PHONY: test
test:
	@echo "Running tests..."
	$(PNPM) run test

## format: Format code with prettier
.PHONY: format
format:
	@echo "Formatting code..."
	$(PNPM) run format
	@echo "Code formatted"

## format-check: Check code formatting without making changes
.PHONY: format-check
format-check:
	@echo "Checking code formatting..."
	$(PNPM) run format-check

## lint: Run linters
.PHONY: lint
lint:
	@echo "Running linters..."
	$(PNPM) run lint

## ci: Run all CI checks (format-check, lint, build, test)
.PHONY: ci
ci: format-check lint build test

## mcp-add: Register this MCP server with Claude Code
.PHONY: mcp-add
mcp-add:
	@echo "Registering $(PACKAGE_NAME) with Claude Code..."
	claude mcp add $(PACKAGE_NAME) -e TS_MCP_SERVICE_URLS=$${TS_MCP_SERVICE_URLS} -- node $(PWD)/dist/index.js
	@echo "Done. Run 'claude mcp list' to verify."

## clean: Remove build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dist
	@rm -rf node_modules
	@rm -rf *.tsbuildinfo
	@echo "Clean complete"
