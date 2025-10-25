# Protobuf compiler installation
PROTOC := $(shell command -v protoc 2> /dev/null)
PROTO_VERSION := 3.21.0
PROTO_GEN_GO := $(shell command -v protoc-gen-go 2> /dev/null)
PROTO_GEN_GO_GRPC := $(shell command -v protoc-gen-go-grpc 2> /dev/null)

.PHONY: deps build run test clean proto

# Default target
all: build run

# Install dependencies
deps:
	go mod tidy
	go mod download
	export PATH=$$(go env GOPATH)/bin:$$PATH
	@if [ -z "$(PROTOC)" ]; then \
		echo "Installing protoc..."; \
		sudo apt-get update && sudo apt-get install -y protobuf-compiler; \
	fi
	@if [ -z "$(PROTO_GEN_GO)" ]; then \
		echo "Installing protoc-gen-go..."; \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@latest; \
	fi
	@if [ -z "$(PROTO_GEN_GO_GRPC)" ]; then \
		echo "Installing protoc-gen-go-grpc..."; \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest; \
	fi

# Verify all dependencies are available
verify:
	@echo "Verifying dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "Go is not installed"; exit 1; }
	@command -v redis-cli >/dev/null 2>&1 || { echo "Redis is not installed"; exit 1; }
	@command -v psql >/dev/null 2>&1 || { echo "PostgreSQL is not installed"; exit 1; }
	@test -d /opt/kafka || { echo "Kafka is not installed"; exit 1; }
	@echo "All dependencies are available"

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/v1/*.proto

# Build all services
build: proto
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