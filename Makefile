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
	@echo "Building Docker image..."
	docker compose build

docker-up:
	@echo "Starting all services..."
	docker compose up -d
	@echo ""
	@echo "Services starting..."
	@echo "  - App:      http://localhost:8081"
	@echo "  - Postgres: localhost:5432"
	@echo "  - Ollama:   http://localhost:11434"
	@echo ""
	@echo "Note: First run will pull the llama2 model (~4GB). Check progress with: make docker-logs-ollama"

docker-down:
	@echo "Stopping all services..."
	docker compose down

docker-restart:
	@echo "Restarting all services..."
	docker compose restart

docker-logs:
	docker compose logs -f

docker-logs-app:
	docker compose logs -f app

docker-logs-ollama:
	docker compose logs -f ollama ollama-pull

docker-logs-db:
	docker compose logs -f postgres

docker-ps:
	docker compose ps

docker-clean:
	@echo "Stopping and removing all containers, networks, and volumes..."
	docker compose down -v --rmi local
	@echo "Clean complete"

docker-shell:
	docker compose exec app sh

docker-rebuild:
	@echo "Rebuilding and restarting app..."
	docker compose up -d --build app

# Pull Ollama model manually
docker-pull-model:
	@echo "Pulling llama2 model..."
	docker compose exec ollama ollama pull llama2

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
	@echo ""
	@echo "Docker (Full Stack):"
	@echo "  docker-build  - Build all Docker images"
	@echo "  docker-up     - Start all services (app, db, ollama)"
	@echo "  docker-down   - Stop all services"
	@echo "  docker-restart- Restart all services"
	@echo "  docker-logs   - View all logs"
	@echo "  docker-logs-app    - View app logs"
	@echo "  docker-logs-ollama - View Ollama logs"
	@echo "  docker-logs-db     - View database logs"
	@echo "  docker-ps     - Show running containers"
	@echo "  docker-clean  - Remove all containers and volumes"
	@echo "  docker-rebuild- Rebuild and restart app"
	@echo "  docker-pull-model - Manually pull llama2 model"

