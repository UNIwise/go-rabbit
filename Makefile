.DEFAULT_GOAL := help

.PHONY: help test test-e2e test-all cover lint fmt vet tidy up down example vuln-check

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

test: ## Run unit tests
	go test -race ./...

test-e2e: ## Run e2e tests (requires Docker, starts RabbitMQ via testcontainers)
	go test -race -tags e2e -count=1 -timeout 10m ./e2e/...

test-all: test test-e2e ## Run unit and e2e tests

cover: ## Run unit tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint: ## Run golangci-lint (including e2e files)
	golangci-lint run --build-tags e2e ./...

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet (including e2e files)
	go vet -tags e2e ./...

tidy: ## Tidy go.mod/go.sum
	go mod tidy

up: ## Start local RabbitMQ (management UI on http://localhost:15672)
	docker compose up -d

down: ## Stop local RabbitMQ
	docker compose down

example: ## Run an example against local RabbitMQ, e.g. make example name=retry
	go run ./examples/$(or $(name),simple)

vuln-check: ## Run vulnerability check
	grype dir:.