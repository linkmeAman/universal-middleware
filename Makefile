.PHONY: deps build run test clean

# Default target
all: build run

# Install dependencies
deps:
	go mod tidy
	go mod download

# Verify all dependencies are available
verify:
	@echo "Verifying dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed"; exit 1; }
	@command -v redis-cli >/dev/null 2>&1 || { echo "Redis is not installed"; exit 1; }
	@command -v psql >/dev/null 2>&1 || { echo "PostgreSQL is not installed"; exit 1; }
	@test -d /opt/kafka || { echo "Kafka is not installed"; exit 1; }
	@echo "All dependencies are available"

# Build all services
build:
	@mkdir -p bin
	go build -o bin/api-gateway ./cmd/api-gateway
	go build -o bin/ws-hub ./cmd/ws-hub
	go build -o bin/command-service ./cmd/command-service
	go build -o bin/processor ./cmd/processor
	go build -o bin/cache-updater ./cmd/cache-updater

# Run infrastructure dependencies
infra-up:
	docker-compose -f deployments/docker/docker-compose.yaml up -d

# Stop infrastructure
infra-down:
	docker-compose -f deployments/docker/docker-compose.yaml down

# Run services
run: infra-up
	@echo "Starting services..."
	./bin/api-gateway & echo $$! > .pid.api-gateway
	./bin/ws-hub & echo $$! > .pid.ws-hub
	./bin/command-service & echo $$! > .pid.command-service
	./bin/processor & echo $$! > .pid.processor
	./bin/cache-updater & echo $$! > .pid.cache-updater
	@echo "Services started"

# Stop services
stop:
	@for pid in .pid.*; do \
		if [ -f "$$pid" ]; then \
			kill -TERM $$(cat "$$pid") 2>/dev/null || true; \
			rm "$$pid"; \
		fi \
	done
	@echo "Services stopped"

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f .pid.*

# Run tests
test:
	go test -v ./...