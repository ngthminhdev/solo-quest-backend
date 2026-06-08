.PHONY: help build run dev test test-e2e db-reset-dev db-version migrate-up migrate-down lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the API binary
	go build -o ./tmp/main ./cmd/api

run: ## Run the API server directly
	go run ./cmd/api/main.go

dev: ## Run with Air hot-reload
	air

test: ## Run unit tests
	go test ./internal/... -count=1 -short

test-e2e: ## Run e2e tests (requires test DB)
	APP_ENV=test go test ./tests/e2e/... -count=1 -v

db-version: ## Show current migration version
	go run ./cmd/migrate/main.go version

migrate-up: ## Run pending migrations
	go run ./cmd/migrate/main.go up

migrate-down: ## Roll back last migration (DANGEROUS)
	go run ./cmd/migrate/main.go down

db-reset-dev: ## Reset local dev database (requires ALLOW_DEV_DB_RESET=true)
	@./scripts/reset_dev_db.sh

lint: ## Run Go vet and staticcheck
	go vet ./...
