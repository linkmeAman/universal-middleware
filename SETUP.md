# Universal Middleware Setup Guide

This comprehensive guide will help you set up and run the Universal Middleware project on a new machine. Follow these steps carefully to ensure a smooth setup process.

## System Requirements

### Hardware Requirements
- Minimum 4GB RAM (8GB recommended)
- 20GB free disk space
- 2 CPU cores (4 recommended)

### Software Prerequisites

1. **Go Installation**
   ```bash
   # Install Go 1.21 or higher
   wget https://go.dev/dl/go1.21.3.linux-amd64.tar.gz
   sudo rm -rf /usr/local/go
   sudo tar -C /usr/local -xzf go1.21.3.linux-amd64.tar.gz
   
   # Add Go to PATH (add to ~/.bashrc)
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
   source ~/.bashrc
   
   # Verify installation
   go version
   ```

2. **PostgreSQL**
   ```bash
   # Install PostgreSQL
   sudo apt-get update
   sudo apt-get install -y postgresql postgresql-contrib
   
   # Start PostgreSQL
   sudo systemctl enable postgresql
   sudo systemctl start postgresql
   
   # Create database and user
   sudo -u postgres psql -c "CREATE DATABASE middleware;"
   sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD 'postgres';"
   ```

3. **Redis**
   ```bash
   # Install Redis
   sudo apt-get install -y redis-server
   
   # Configure Redis to start on boot
   sudo systemctl enable redis-server
   sudo systemctl start redis-server
   
   # Test Redis
   redis-cli ping  # Should return PONG
   ```

4. **Protocol Buffers Compiler (protoc)**
   ```bash
   # Install protoc
   sudo apt-get install -y protobuf-compiler
   
   # Install Go protobuf plugins
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   
   # Verify installation
   protoc --version
   ```

5. **Open Policy Agent (OPA)**
   ```bash
   curl -L -o opa https://openpolicyagent.org/downloads/v0.42.0/opa_linux_amd64
   chmod 755 opa
   sudo mv opa /usr/local/bin/
   ```

## Project Setup

1. **Clone the Repository**
   ```bash
   git clone https://github.com/linkmeAman/universal-middleware.git
   cd universal-middleware
   ```

2. **Environment Setup**
   ```bash
   # Create config directory if it doesn't exist
   mkdir -p config
   
   # Copy example configurations
   cp config/config.yaml.example config/config.yaml
   cp config/auth.yaml.example auth.yaml
   cp config/backends.yaml.example backends.yaml
   ```

3. **Install Go Dependencies**
   ```bash
   go mod download
   go mod verify
   ```

4. **Database Migrations**
   ```bash
   # Run all migrations
   make migrate-up
   ```

## Configuration

1. **Required Environment Variables**
   Add these to your ~/.bashrc or environment:
   ```bash
   export MIDDLEWARE_ENV=development
   export MIDDLEWARE_CONFIG_PATH=/path/to/universal-middleware/config
   export MIDDLEWARE_LOG_LEVEL=debug
   export MIDDLEWARE_DB_HOST=localhost
   export MIDDLEWARE_DB_PORT=5432
   export MIDDLEWARE_DB_NAME=middleware
   export MIDDLEWARE_DB_USER=postgres
   export MIDDLEWARE_DB_PASSWORD=postgres
   export MIDDLEWARE_REDIS_HOST=localhost
   export MIDDLEWARE_REDIS_PORT=6379
   ```

2. **Service Ports**
   Make sure these ports are available:
   - API Gateway: 8080
   - WebSocket Hub: 8085
   - Command Service: 8082
   - Processor: 8083
   - Cache Updater: 8084
   - Redis: 6379
   - PostgreSQL: 5432
   - OPA: 8181

## Building and Running

1. **Build Services**
   ```bash
   # Build all services
   make build
   ```

2. **Start Services**
   ```bash
   # Method 1: Using start script (Recommended)
   ./scripts/start-services.sh
   
   # Method 2: Manual startup
   ./bin/api-gateway &
   ./bin/ws-hub &
   ./bin/command-service &
   ./bin/processor &
   ./bin/cache-updater &
   ```

3. **Verify Services**
   ```bash
   # Check service health
   curl http://localhost:8080/health  # API Gateway
   curl http://localhost:8085/health  # WebSocket Hub
   curl http://localhost:8082/health  # Command Service
   curl http://localhost:8083/health  # Processor
   curl http://localhost:8084/health  # Cache Updater
   ```

## Troubleshooting

### Common Issues and Solutions

1. **protoc-gen-go not found**
   ```bash
   # Solution: Add Go bin to PATH
   echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
   source ~/.bashrc
   ```

2. **Database Connection Issues**
   ```bash
   # Check PostgreSQL status
   sudo systemctl status postgresql
   
   # Check logs
   sudo tail -f /var/log/postgresql/postgresql-15-main.log
   ```

3. **Redis Connection Issues**
   ```bash
   # Check Redis status
   sudo systemctl status redis-server
   
   # Check Redis connectivity
   redis-cli ping
   ```

4. **Service Start Failure**
   - Check logs in `/var/log/universal-middleware/`
   - Verify all required services are running
   - Ensure correct permissions on config files
   - Verify port availability: `sudo lsof -i :<port>`

### Log Locations
- Application logs: `/var/log/universal-middleware/`
- PostgreSQL logs: `/var/log/postgresql/`
- Redis logs: `/var/log/redis/`
- Service-specific logs: `logs/` directory in project root

## Maintenance

1. **Updating the Project**
   ```bash
   git pull origin main
   make build
   make migrate-up
   ./scripts/start-services.sh
   ```

2. **Cleaning Up**
   ```bash
   # Stop all services
   pkill -f 'bin/(api-gateway|ws-hub|command-service|processor|cache-updater)'
   
   # Clean build artifacts
   make clean
   ```

3. **Backup Database**
   ```bash
   pg_dump -U postgres middleware > backup.sql
   ```

## Additional Notes

1. **Development Tools**
   - Install recommended VS Code extensions
   - Configure linters and formatters
   - Set up Git hooks for pre-commit checks

2. **Security Considerations**
   - Change default passwords in production
   - Configure firewalls appropriately
   - Use TLS for all external communications
   - Regularly update dependencies

3. **Performance Tuning**
   - Adjust PostgreSQL configuration based on hardware
   - Configure Redis memory limits
   - Monitor system resources

## Support

For additional help:
1. Check the project documentation in `/docs`
2. Review the development guide in `DEVELOPMENT.md`
3. Submit issues through the project's issue tracker
4. Include relevant logs and error messages when reporting issues
