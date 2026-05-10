# Docker Hub configuration
DOCKER_REGISTRY ?= cameronx
RELEASE_MANIFEST := .github/release-please/.release-please-manifest.json

# TLS Certificate Configuration
CERTS_DIR := certs
TLS_CERT := $(CERTS_DIR)/otel-collector.crt
TLS_KEY := $(CERTS_DIR)/otel-collector.key
TLS_CA := $(CERTS_DIR)/ca.crt

# Services to test (add new services here - must match docker-compose service name)
ALL_SERVICES := go-gin py-fastapi cs-aspnet

# MCP servers (separate from ALL_SERVICES because path differs: mcp/<lang> vs services/<name>)
MCP_SERVERS := ts-mcp

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## up: Start development environment (use SERVICES=go-gin to include services)
.PHONY: up
up: create-dev-env generate-certs
	@files="-f docker-compose.yml"; \
	if [ -n "$(SERVICES)" ]; then \
		for p in $$(echo "$(SERVICES)" | tr ',' ' '); do \
			files="$$files -f docker-compose.$$p.yml"; \
		done; \
	fi; \
	echo "Starting development environment..."; \
	docker compose $$files up --remove-orphans --build -d

## down: Stop all containers
.PHONY: down
down:
	@echo "Stopping development environment..."
	@docker compose down -v --remove-orphans

.PHONY: create-dev-env
create-dev-env:
	@if [ ! -f .env ]; then \
		echo "Creating .env from template..."; \
		cp .env.example .env; \
	else \
		echo ".env already exists"; \
	fi

.PHONY: generate-certs
generate-certs:
	@if [ ! -d "$(CERTS_DIR)" ]; then \
		echo "Creating certificates directory..."; \
		mkdir -p $(CERTS_DIR); \
	fi
	@if [ ! -f "$(TLS_CERT)" ] || [ ! -f "$(TLS_KEY)" ]; then \
		echo "Generating self-signed certificates..."; \
		openssl req -x509 -newkey rsa:4096 -nodes \
			-keyout $(TLS_KEY) \
			-out $(TLS_CERT) \
			-days 365 \
			-subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=otel-collector" \
			-addext "subjectAltName=DNS:localhost,DNS:lgtm,IP:127.0.0.1"; \
		cp $(TLS_CERT) $(TLS_CA); \
		echo "Certificates generated successfully"; \
		echo "  Certificate: $(TLS_CERT)"; \
		echo "  Key: $(TLS_KEY)"; \
		echo "  CA: $(TLS_CA)"; \
	else \
		echo "Certificates already exist"; \
	fi

## build: Build all service Docker images (use -j for parallel)
.PHONY: build
build: $(foreach s,$(ALL_SERVICES),docker-build-$(s)) $(foreach m,$(MCP_SERVERS),docker-build-mcp-$(m))

## docker-build-mcp-%: Build Docker image for an MCP server (e.g., make docker-build-mcp-ts-mcp)
.PHONY: docker-build-mcp-%
docker-build-mcp-%:
	@$(eval MCP_LANG := $(shell echo $* | sed 's/-mcp//'))
	@$(MAKE) -C mcp/$(MCP_LANG) docker-build

## docker-build-%: Build Docker image for a specific service (e.g., make docker-build-go-gin ARCH=amd64)
.PHONY: docker-build-%
docker-build-%:
	@$(MAKE) -C services/$* docker-build

## docker-publish: Build and publish all service Docker images to registry
.PHONY: docker-publish
docker-publish: $(foreach s,$(ALL_SERVICES),docker-publish-$(s))

## docker-publish-%: Build and publish a service Docker image (e.g., make docker-publish-go-gin)
.PHONY: docker-publish-%
docker-publish-%:
	@SVC_VERSION=$$(jq -r '."services/$*"' $(RELEASE_MANIFEST)); \
	echo "Publishing $* version $${SVC_VERSION}..."; \
	$(MAKE) -C services/$* docker-publish VERSION=$${SVC_VERSION} IMAGE=$(DOCKER_REGISTRY)/otel-polyglot-$*

## ci: Run CI checks for all services in Docker (use -j for parallel, e.g., make -j ci)
.PHONY: ci
ci: create-dev-env generate-certs $(foreach s,$(ALL_SERVICES),ci-$(s)) $(foreach m,$(MCP_SERVERS),ci-$(m))

## ci-%: Run CI checks for a specific service in Docker (e.g., make ci-go-gin)
.PHONY: ci-%
ci-%:
	@echo "Running $* CI checks in Docker..."
	@docker compose -f docker-compose.yml -f docker-compose.$*.yml run --rm --no-deps --entrypoint "" --build $* make ci

## ci-mcp: Run CI checks for all MCP servers (use -j for parallel)
.PHONY: ci-mcp
ci-mcp: create-dev-env generate-certs $(foreach m,$(MCP_SERVERS),ci-$(m))

## test-integration: Build images and start LGTM + all services for integration testing
.PHONY: test-integration
test-integration: create-dev-env generate-certs build
	@echo "Starting integration test environment..."
	docker compose -f docker-compose.yml -f docker-compose.test-integration.yml up --remove-orphans -d

## lint-actions: Lint GitHub Actions workflows
.PHONY: lint-actions
lint-actions:
	@actionlint
