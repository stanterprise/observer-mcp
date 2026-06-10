GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
	GOBIN := $(shell go env GOPATH)/bin
endif

BIN_DIR := bin
SERVER_BIN := $(BIN_DIR)/observer-mcp
OBSERVER_DOCKER_NETWORK ?= observer_observer-net
OBSERVER_COMPOSE_FILE := docker-compose.observer.yml

.PHONY: help build build-dev test fmt vet tidy clean docker-build mcp-up-observer mcp-down-observer mcp-logs-observer

.DEFAULT_GOAL := help

help:
	@awk 'BEGIN {FS = ":.*##"}; /^[a-zA-Z0-9_.-]+:.*##/ { printf "\033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build observer-mcp binary
	@mkdir -p $(BIN_DIR)
	go build -o $(SERVER_BIN) ./cmd/server

build-dev: build ## Build and verify bin/observer-mcp is ready for Copilot MCP use
	@echo "Binary ready at $(SERVER_BIN)"
	@echo "Start Observer (make web-dev-mode in observer repo), then open this workspace in VS Code."
	@echo "Copilot will pick up .vscode/mcp.json automatically."

test: ## Run unit tests
	go test ./...

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy module dependencies
	go mod tidy

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out

docker-build: ## Build container image locally
	docker build -t observer-mcp:local .

mcp-up-observer: ## Start observer-mcp in Observer docker compose network
	@docker network inspect $(OBSERVER_DOCKER_NETWORK) >/dev/null 2>&1 || (echo "Network $(OBSERVER_DOCKER_NETWORK) not found. Start Observer first (for example: make web-dev-mode in observer)." && exit 1)
	OBSERVER_DOCKER_NETWORK=$(OBSERVER_DOCKER_NETWORK) docker compose -f $(OBSERVER_COMPOSE_FILE) up -d --build

mcp-down-observer: ## Stop observer-mcp from Observer docker compose network
	OBSERVER_DOCKER_NETWORK=$(OBSERVER_DOCKER_NETWORK) docker compose -f $(OBSERVER_COMPOSE_FILE) down

mcp-logs-observer: ## Tail observer-mcp logs when running in Observer docker compose network
	OBSERVER_DOCKER_NETWORK=$(OBSERVER_DOCKER_NETWORK) docker compose -f $(OBSERVER_COMPOSE_FILE) logs -f observer-mcp
