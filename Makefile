# TLS Certificate Configuration
CERTS_DIR := certs
TLS_CERT := $(CERTS_DIR)/otel-collector.crt
TLS_KEY := $(CERTS_DIR)/otel-collector.key
TLS_CA := $(CERTS_DIR)/ca.crt

# Services to test (add new services here - must match docker-compose service name)
SERVICES := go-gin

## help: Display this help message
.PHONY: help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## up: Start development environment (use PROFILES=go-gin to include services)
.PHONY: up
up: create-dev-env generate-certs
	@profiles="default"; \
	if [ -n "$(PROFILES)" ]; then \
		profiles="$$(echo "$(PROFILES)" | tr ',' ' ') default"; \
	fi; \
	echo "Starting development environment (profiles: $$profiles)..."; \
	docker compose $$(for p in $$profiles; do echo "--profile $$p"; done) up --build -d

## down: Stop all containers
.PHONY: down
down:
	@echo "Stopping development environment..."
	@docker compose --profile "*" down -v

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

## build: Build all service Docker images
.PHONY: build
build:
	@echo "Building all service images..."
	@for service in $(SERVICES); do \
		echo "\n=== Building $$service ===" ; \
		$(MAKE) docker-build-$$service ; \
	done

## docker-build-%: Build Docker image for a specific service (e.g., make docker-build-go-gin ARCH=amd64)
.PHONY: docker-build-%
docker-build-%:
	@$(MAKE) -C services/$* docker-build

## ci: Run CI checks for all services in Docker
.PHONY: ci
ci: create-dev-env generate-certs
	@echo "Running CI checks for all services..."
	@for service in $(SERVICES); do \
		echo "\n=== Checking $$service ===" ; \
		$(MAKE) ci-$$service || exit 1 ; \
	done
	@echo "\n=== All CI checks passed ==="

## ci-%: Run CI checks for a specific service in Docker (e.g., make ci-go-gin)
.PHONY: ci-%
ci-%:
	@echo "Running $* CI checks in Docker..."
	@docker compose --profile $* --profile default run --rm --no-deps --entrypoint "" --build $* make ci

## lint-actions: Lint GitHub Actions workflows
.PHONY: lint-actions
lint-actions:
	@actionlint
