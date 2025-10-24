# Local Development Setup Guide

This guide provides comprehensive instructions for setting up and running the Universal Middleware project locally without Docker.

## Project Overview

The Universal Middleware project consists of several microservices:
- API Gateway: Main entry point for HTTP requests
- WebSocket Hub: Handles real-time WebSocket connections
- Command Service: Processes business commands
- Cache Updater: Maintains cache consistency
- Processor: Handles background processing tasks

## Prerequisites

### Required Software
1. Go 1.21+ 
   ```bash
   # Check Go version
   go version
   ```

2. Redis Server (Local)
   ```bash
   # Ubuntu/Debian
   sudo apt-get install redis-server
   sudo systemctl start redis-server
   
   # Check Redis status
   redis-cli ping
   ```

3. PostgreSQL (Local)
   ```bash
   # Ubuntu/Debian
   sudo apt-get install postgresql postgresql-contrib
   sudo systemctl start postgresql
   
   # Create database
   sudo -u postgres createdb middleware
   sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD 'postgres';"
   ```

4. Open Policy Agent (OPA)
   ```bash
   # Download OPA binary
   curl -L -o opa https://openpolicyagent.org/downloads/v0.42.0/opa_linux_amd64
   chmod 755 opa
   sudo mv opa /usr/local/bin/
   
   # Run OPA server
   opa run --server --addr :8181
   ```

## Service Configuration

### Redis Configuration
- Default port: 6379
- No authentication (development mode)
- Configure in config.yaml:
  ```yaml
  redis:
    addresses:
      - localhost:6379
    password: ""
  ```

### PostgreSQL Configuration
- Default port: 5432
- Database: middleware
- User: postgres
- Password: postgres
- Configure in config.yaml:
  ```yaml
  database:
    primary:
      host: localhost
      port: 5432
      database: middleware
      username: postgres
      password: postgres
  ```

### OPA Configuration
- Default port: 8181
- Configure in config.yaml:
  ```yaml
  auth:
    opa_endpoint: http://localhost:8181
    opa_policy: middleware/authz
  ```

## Development Workflow

1. Start Required Services
   ```bash
   # Start Redis
   sudo systemctl start redis-server
   
   # Start PostgreSQL
   sudo systemctl start postgresql
   
   # Start OPA
   opa run --server --addr :8181 &
   ```

2. Run the Application
   ```bash
   # API Gateway
   go run cmd/api-gateway/main.go
   
   # WebSocket Hub (in another terminal)
   go run cmd/ws-hub/main.go
   ```

## Checking Service Status

1. Redis
   ```bash
   redis-cli ping  # Should return PONG
   ```

2. PostgreSQL
   ```bash
   pg_isready  # Should return accepting connections
   ```

3. OPA
   ```bash
   curl http://localhost:8181/health  # Should return {"status":"ok"}
   ```

## Building and Running Services

### Build All Services
```bash
# Build all services
make build

# Or build individual services
go build -o bin/api-gateway cmd/api-gateway/main.go
go build -o bin/ws-hub cmd/ws-hub/main.go
go build -o bin/command-service cmd/command-service/main.go
go build -o bin/processor cmd/processor/main.go
go build -o bin/cache-updater cmd/cache-updater/main.go
```

### Running Services in Development Mode
```bash
# Option 1: Start all services using the script (recommended)
./scripts/start-services.sh

# Option 2: Start services individually in separate terminals
./bin/api-gateway     # REST API and main entry point (Port 8080)
./bin/ws-hub         # WebSocket hub for real-time updates (Port 8081)
./bin/command-service # Command processing service (Port 8082)
./bin/processor      # Event processor service (Port 8083)
./bin/cache-updater  # Cache invalidation service (Port 8084)

# Each service exposes a /health endpoint for monitoring
curl http://localhost:8080/health  # Check API Gateway health
curl http://localhost:8081/health  # Check WebSocket Hub health
curl http://localhost:8082/health  # Check Command Service health
curl http://localhost:8083/health  # Check Processor health
curl http://localhost:8084/health  # Check Cache Updater health
```

Health check response format:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "services": {
    "database": "healthy",
    "redis": "healthy",
    "kafka": "healthy",
    "processor": "healthy"
  }
}
```

## Testing

### Run All Tests
```bash
make test

# Run specific test packages
go test ./internal/api/...
go test ./internal/command/...
```

### Integration Tests
```bash
make integration-test
```

### Load Tests
```bash
make load-test
```

## Troubleshooting

### Redis
- Check logs: `sudo journalctl -u redis-server`
- Configuration file: `/etc/redis/redis.conf`
- Reset Redis: `sudo systemctl restart redis-server`
- Common Issues:
  - Port conflicts: Check if port 6379 is already in use
  - Memory issues: Check Redis memory usage with `redis-cli info memory`
  - Connection refused: Verify Redis is running and listening on the correct interface

### PostgreSQL
- Check logs: `sudo journalctl -u postgresql`
- Configuration: `/etc/postgresql/[version]/main/postgresql.conf`
- Reset PostgreSQL: `sudo systemctl restart postgresql`
- Common Issues:
  - Authentication failures: Check pg_hba.conf
  - Connection limits: Review max_connections setting
  - Disk space: Ensure sufficient space for logs and data

### OPA
- Check OPA status: `curl http://localhost:8181/health`
- Load policy: `curl -X PUT http://localhost:8181/v1/policies/middleware/authz --data-binary @internal/auth/policies/authz.rego`
- Common Issues:
  - Policy loading failures: Verify Rego syntax
  - Permission issues: Check file access rights
  - Memory constraints: Monitor OPA process memory usage

### Application-wide Issues
1. Network Issues
   - Check firewall settings
   - Verify all ports are accessible
   - Test inter-service communication

2. Performance Issues
   - Use `go tool pprof` for profiling
   - Monitor system resources
   - Check for memory leaks

3. Configuration Issues
   - Validate config.yaml syntax
   - Check environment variables
   - Verify file permissions

## Maintenance

### Database Migrations
```bash
# Create new migration
make new-migration name=migration_name

# Run migrations
make migrate-up

# Rollback migrations
make migrate-down
```

### Code Quality
```bash
# Run linter
make lint

# Format code
make fmt

# Generate protobuf
make proto
```

### Monitoring
- Metrics endpoint: http://localhost:8080/metrics
- Health checks: http://localhost:8080/health
- Debug pprof: http://localhost:8080/debug/pprof/

## Security Best Practices

1. Development Environment
   - Use strong passwords for services
   - Keep all services updated
   - Don't expose services to public network
   - Use HTTPS for external communications

2. Code Security
   - Never commit secrets to version control
   - Use environment variables for sensitive data
   - Implement proper input validation
   - Follow secure coding guidelines

## Getting Help

1. Documentation
   - Check `/docs` directory for detailed documentation
   - Review inline code comments
   - Consult API documentation

2. Logs
   - Application logs in /var/log/universal-middleware/
   - Service-specific logs in respective directories
   - System logs via journalctl

3. Support
   - Submit issues through project tracker
   - Check existing issues for similar problems
   - Include relevant logs and error messages