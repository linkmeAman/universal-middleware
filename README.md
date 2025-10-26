# Universal Middleware

A high-performance, cache-first, event-driven middleware platform built in Go.

## Features

- 🚀 Cache-first architecture with Redis
- 🔄 Event-driven command processing with Kafka
- 🔒 OAuth2/OIDC + OPA-based authorization
- 📡 Real-time updates via WebSocket/SSE
- 📊 Comprehensive observability (metrics, logging, tracing)
- ⚡ High-performance API Gateway (HTTP/gRPC)

## Prerequisites

- Go 1.21+
- Redis 7.0+
- Kafka 3.0+
- OPA (Open Policy Agent)
- PostgreSQL 15+ (optional)

## Quick Start

1. **Clone the repository**
```bash
git clone https://github.com/linkmeAman/universal-middleware.git
cd universal-middleware
```

2. **Install dependencies**
```bash
go mod tidy
```

3. **Configure the application**
```bash
cp config.yaml config/config.yaml
# Edit config/config.yaml with your settings
```

4. **Run required services**
```bash
# Start Redis
docker run -d --name redis -p 6379:6379 redis:7

# Start Kafka (if using event processing)
docker-compose up -d kafka

# Start OPA (if using authorization)
docker run -d --name opa -p 8181:8181 openpolicyagent/opa:latest run --server
```

5. **Build and start all services**
```bash
# Build all services
make build

# Start all services in the correct order
./scripts/start-services.sh

# Or start services individually:
./bin/api-gateway      # Port 8080 - HTTP/gRPC API Gateway
./bin/ws-hub          # Port 8085 - WebSocket Hub with Redis pub/sub
./bin/command-service  # Port 8082 - Command processing service
./bin/processor        # Port 8083 - Event processor service
./bin/cache-updater    # Port 8084 - Cache management service
```

## Project Structure

```
universal-middleware/
├── cmd/                  # Application entry points
├── internal/            # Private application code
│   ├── api/            # API handlers and middleware
│   ├── cache/          # Cache implementation
│   ├── command/        # Command processing
│   └── events/         # Event handling
├── pkg/                # Public shared packages
└── test/              # Integration and load tests
```

## Configuration

The application uses a layered configuration approach:

### 1. YAML Configuration
- Main config: `./config/config.yaml`
- Auth config: `./config/auth.yaml`
- Backend config: `./config/backends.yaml`

### 2. Environment Variables
- Prefix: `UMW_`
- Example: `UMW_SERVER_PORT=9090`

### 3. Security Configuration
- JWT secret (required): `UMW_JWT_SECRET`
- Session secret (required): `UMW_SESSION_SECRET`
- OAuth2 settings (optional):
  - `UMW_OAUTH2_PROVIDER_URL`
  - `UMW_OAUTH2_CLIENT_ID`
  - `UMW_OAUTH2_CLIENT_SECRET`

## API Documentation

### RESTful Endpoints

#### System Endpoints
- `GET /health` - Health check with dependency status
- `GET /metrics` - Prometheus metrics

#### Command Endpoints
- `POST /v1/commands` - Submit new command
- `GET /v1/commands/{id}` - Get command status

#### WebSocket Endpoints
- `GET /ws` - WebSocket connection with JWT auth
- `GET /ws/health` - WebSocket hub health check

#### Authentication Endpoints
- `GET /api/v1/auth/login` - OAuth2 login
- `GET /api/v1/auth/callback` - OAuth2 callback
- `GET /api/v1/auth/logout` - Logout
- `GET /api/v1/userinfo` - Get user info
- `POST /api/v1/refresh` - Refresh JWT token

### Middleware Chain

1. Request ID
2. Real IP
3. Recovery
4. Timeout (30s)
5. Request Logger
6. Request Tracker
7. Metrics Collector
8. Rate Limiter (Redis-based)
9. Authentication (JWT)
10. Authorization (OPA)
11. Validation

Note: Rate limiting is applied selectively based on endpoint requirements. Command endpoints have their own rate limiting rules.

### Request Validation

Example of validated endpoint:

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=50"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}
```

## Development

### Running Tests
```bash
go test ./...
```

### Adding New Endpoints

1. Create handler in `internal/api/handlers/`
2. Define validation struct if needed
3. Add route in `cmd/api-gateway/main.go`
4. Add tests in `test/`

### Authorization Policies

OPA policies are stored in `internal/auth/policies/` and loaded at startup.

## Monitoring & Observability

- Metrics: Prometheus endpoint at `/metrics`
- Logging: Structured JSON logs using Zap
- Tracing: OpenTelemetry integration

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.