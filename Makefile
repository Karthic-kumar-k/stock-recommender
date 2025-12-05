.PHONY: build run test clean dev lint fmt

# Binary name
BINARY_NAME=recommender
BINARY_PATH=bin/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt

# Main package
MAIN_PACKAGE=./cmd/recommender

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_PATH)"

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GORUN) $(MAIN_PACKAGE)

# Run with config file
run-config:
	@echo "Running $(BINARY_NAME) with config..."
	$(GORUN) $(MAIN_PACKAGE) -config configs/config.yaml

# Development mode with hot reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# Database setup with Docker
db-up:
	@echo "Starting PostgreSQL..."
	docker compose up -d
	@echo "Waiting for database to be ready..."
	@sleep 3
	@echo "PostgreSQL is ready"

db-down:
	@echo "Stopping PostgreSQL..."
	docker compose down

db-reset:
	@echo "Resetting database..."
	docker compose down -v
	docker compose up -d
	@sleep 3
	@echo "Database reset complete"

db-logs:
	docker compose logs -f postgres

# Docker commands
docker-build:
	docker build -t stock-recommender .

docker-run:
	docker run -p 8080:8080 --env-file .env stock-recommender

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  run-config    - Run with config file"
	@echo "  dev           - Run in development mode with hot reload"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download dependencies"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  db-up         - Start PostgreSQL with Docker"
	@echo "  db-down       - Stop PostgreSQL"
	@echo "  db-reset      - Reset database (removes data)"
	@echo "  db-logs       - View PostgreSQL logs"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"

