# Variables
PROJECT=cs-aspnet
VERSION?=0.1.0
IMAGE ?= $(PROJECT)
SRC_DIR=src/CsAspNet
TEST_DIR=tests/CsAspNet.Tests
ARTIFACTS_DIR=artifacts

# Docker build platform (default: amd64, override with ARCH=arm64)
ARCH ?= amd64
PLATFORM = linux/$(ARCH)

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: Publish application to artifacts/publish
.PHONY: build
build:
	@echo "Publishing $(PROJECT)..."
	dotnet publish $(SRC_DIR) -c Release -o $(ARTIFACTS_DIR)/publish
	@echo "Publish complete: $(ARTIFACTS_DIR)/publish"

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

## run: Run the service on port 8080
.PHONY: run
run:
	@echo "Running $(PROJECT)..."
	dotnet run --project $(SRC_DIR) --launch-profile http

## test: Run tests with coverage report
.PHONY: test
test:
	@echo "Running tests with coverage..."
	@rm -rf $(ARTIFACTS_DIR)/*/coverage.cobertura.xml $(ARTIFACTS_DIR)/coverage
	dotnet tool restore
	dotnet test $(TEST_DIR) -v n --collect:"XPlat Code Coverage" \
		--settings .runsettings \
		--results-directory $(ARTIFACTS_DIR) \
		--logger "console;verbosity=detailed"
	@echo ""
	@echo "Coverage summary:"
	dotnet reportgenerator \
		-reports:"$(ARTIFACTS_DIR)/**/coverage.cobertura.xml" \
		-targetdir:"$(ARTIFACTS_DIR)/coverage" \
		-reporttypes:TextSummary
	@cat $(ARTIFACTS_DIR)/coverage/Summary.txt

## format: Format code with dotnet format
.PHONY: format
format:
	@echo "Formatting code..."
	dotnet format $(SRC_DIR)
	@echo "Code formatted"

## format-check: Check formatting without making changes
.PHONY: format-check
format-check:
	@echo "Checking formatting..."
	dotnet format $(SRC_DIR) --verify-no-changes

## lint: Run linters (format check + style analysis)
.PHONY: lint
lint: format-check
	@echo "Running style analysis..."
	dotnet format analyzers $(SRC_DIR) --verify-no-changes

## security: Check for vulnerable NuGet packages
.PHONY: security
security:
	@echo "Checking for vulnerable packages..."
	dotnet list $(SRC_DIR) package --vulnerable --include-transitive

## ci: Run all CI checks (lint, security, test)
.PHONY: ci
ci: lint security test

## clean: Remove build artifacts
.PHONY: clean
clean:
	@echo "Cleaning $(ARTIFACTS_DIR)..."
	@rm -rf $(ARTIFACTS_DIR)
	dotnet clean --nologo -v q
	@echo "Clean complete"
