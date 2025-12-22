# Load environment variables from .env
-include .env
export

.PHONY: build run test test-e2e test-e2e-create test-e2e-get test-e2e-list test-e2e-update test-e2e-delete test-e2e-publish test-e2e-schedule test-e2e-draft lint clean deps dev migrate-up migrate-down migrate-status migrate-create docker-build docker-up docker-down docker-restart docker-rebuild docker-logs docker-ps

# Binary name
BINARY_NAME=neo-metric
BUILD_DIR=./bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Build flags
LDFLAGS=-ldflags "-w -s"

## build: Build the application
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api

## run: Run the application
run: build
	@echo "Running..."
	@$(BUILD_DIR)/$(BINARY_NAME)

## dev: Run with hot reload (requires air)
dev:
	@echo "Starting development server..."
	$(GORUN) ./cmd/api

## test: Run tests
test:
	@echo "Testing..."
	$(GOTEST) -v -race ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Testing with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## test-e2e: Run all e2e tests (requires server to be running on localhost:8080)
test-e2e:
	@echo "Running all e2e tests..."
	$(GOTEST) -v ./tests/e2e/...

## test-e2e-create: Run e2e tests for POST /publications
test-e2e-create:
	@echo "Running e2e tests for Create endpoint..."
	$(GOTEST) -v -run TestPublicationCreate ./tests/e2e/...

## test-e2e-get: Run e2e tests for GET /publications/{id}
test-e2e-get:
	@echo "Running e2e tests for Get endpoint..."
	$(GOTEST) -v -run TestPublicationGet ./tests/e2e/...

## test-e2e-list: Run e2e tests for GET /publications
test-e2e-list:
	@echo "Running e2e tests for List endpoint..."
	$(GOTEST) -v -run TestPublicationList ./tests/e2e/...

## test-e2e-update: Run e2e tests for PUT /publications/{id}
test-e2e-update:
	@echo "Running e2e tests for Update endpoint..."
	$(GOTEST) -v -run TestPublicationUpdate ./tests/e2e/...

## test-e2e-delete: Run e2e tests for DELETE /publications/{id}
test-e2e-delete:
	@echo "Running e2e tests for Delete endpoint..."
	$(GOTEST) -v -run TestPublicationDelete ./tests/e2e/...

## test-e2e-publish: Run e2e tests for POST /publications/{id}/publish
test-e2e-publish:
	@echo "Running e2e tests for Publish endpoint..."
	$(GOTEST) -v -run TestPublicationPublish ./tests/e2e/...

## test-e2e-schedule: Run e2e tests for POST /publications/{id}/schedule
test-e2e-schedule:
	@echo "Running e2e tests for Schedule endpoint..."
	$(GOTEST) -v -run TestPublicationSchedule ./tests/e2e/...

## test-e2e-draft: Run e2e tests for POST /publications/{id}/draft
test-e2e-draft:
	@echo "Running e2e tests for Draft endpoint..."
	$(GOTEST) -v -run TestPublicationDraft ./tests/e2e/...

## lint: Run linters
lint:
	@echo "Linting..."
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting..."
	$(GOFMT) ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Database settings
MIGRATIONS_DIR=./migrations

## migrate-up: Run all pending migrations
migrate-up:
	@echo "Running migrations..."
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

## migrate-down: Rollback the last migration
migrate-down:
	@echo "Rolling back last migration..."
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

## migrate-status: Show migration status
migrate-status:
	@echo "Migration status:"
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

## migrate-create: Create a new migration (usage: make migrate-create name=create_users)
migrate-create:
	@echo "Creating migration $(name)..."
	goose -dir $(MIGRATIONS_DIR) create $(name) sql

# Docker settings
DOCKER_COMPOSE_API=docker-compose -f docker-compose.api.yml
DOCKER_IMAGE_NAME=neo-metric-api

## docker-build: Build the API Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE_NAME) .

## docker-up: Start the API container
docker-up:
	@echo "Starting API container..."
	$(DOCKER_COMPOSE_API) up -d
	@echo "API is running at http://localhost:$${SERVER_PORT:-8080}"

## docker-down: Stop the API container
docker-down:
	@echo "Stopping API container..."
	$(DOCKER_COMPOSE_API) down

## docker-restart: Restart the API container
docker-restart:
	@echo "Restarting API container..."
	$(DOCKER_COMPOSE_API) restart

## docker-rebuild: Rebuild and restart API (use after git pull)
docker-rebuild:
	@echo "Rebuilding and restarting API..."
	$(DOCKER_COMPOSE_API) down
	$(DOCKER_COMPOSE_API) build --no-cache
	$(DOCKER_COMPOSE_API) up -d
	@echo "API rebuilt and running at http://localhost:$${SERVER_PORT:-8080}"

## docker-logs: Show API container logs
docker-logs:
	$(DOCKER_COMPOSE_API) logs -f

## docker-ps: Show API container status
docker-ps:
	$(DOCKER_COMPOSE_API) ps

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

# Default target
.DEFAULT_GOAL := help
