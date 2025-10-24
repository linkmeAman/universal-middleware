# Universal Middleware Operations Guide

## Service Architecture

### Core Services
1. **API Gateway** (Port 8080)
   - Main entry point for HTTP/REST requests
   - Handles routing and request validation
   - Implements authentication and authorization
   - Provides health monitoring of all services
   - Exposes Prometheus metrics endpoint

2. **WebSocket Hub** (Port 8081)
   - Manages real-time WebSocket connections
   - Handles client message broadcasting
   - Maintains connection state

3. **Command Service** (Port 8082)
   - Processes business commands
   - Validates command payload
   - Ensures command idempotency

4. **Cache Updater** (Port 8083)
   - Maintains cache consistency
   - Listens for cache invalidation events
   - Updates Redis cache based on events

5. **Processor** (Port 8084)
   - Handles background processing
   - Processes Kafka events
   - Manages async workflows

## Monitoring and Debugging

### Health Checks
Each service exposes a comprehensive health endpoint at `/health` that monitors all dependencies:
```bash
# Check individual services
curl http://localhost:8080/health  # API Gateway
curl http://localhost:8081/health  # WebSocket Hub
curl http://localhost:8082/health  # Command Service
curl http://localhost:8083/health  # Event Processor
curl http://localhost:8084/health  # Cache Updater
```

Health check response format:
```json
{
  "status": "healthy|degraded",
  "version": "1.0.0",
  "services": {
    "database": "healthy|unhealthy: error message",
    "redis": "healthy|unhealthy: error message",
    "kafka": "healthy|unhealthy: error message",
    "processor": "healthy|unhealthy: error message"
  }
}
```

The status will be:
- `healthy`: All dependencies are working correctly
- `degraded`: One or more dependencies have issues but service is still operational

### Metrics
Prometheus metrics are available at `/metrics` on each service. Key metrics include:

1. **Service Health**
   - `service_health_status`: Overall service health (1=healthy, 0=degraded)
   - `dependency_health_status{dependency="name"}`: Individual dependency health
   - `health_check_duration_seconds`: Health check execution time

2. **HTTP Metrics**
   - `http_request_duration_seconds`: Request latency histogram
   - `http_requests_total{code="status"}`: Request count by status code
   - `http_request_size_bytes`: Request size distribution

3. **Cache Metrics**
   - `cache_operation_duration_seconds{operation="type"}`: Cache operation latency
   - `cache_hits_total`: Number of cache hits
   - `cache_misses_total`: Number of cache misses

4. **Queue Metrics**
   - `kafka_message_lag{topic="name"}`: Consumer lag by topic
   - `kafka_message_processing_duration_seconds`: Message processing time
   - `kafka_messages_processed_total{status="success|error"}`: Processed message count

Access metrics:
```bash
curl -s http://localhost:8080/metrics | grep ^service  # Service metrics
curl -s http://localhost:8080/metrics | grep ^http     # HTTP metrics
curl -s http://localhost:8080/metrics | grep ^cache    # Cache metrics
curl -s http://localhost:8080/metrics | grep ^kafka    # Kafka metrics
```

### Logging
- **Log Levels**: ERROR, WARN, INFO, DEBUG
- **Log Format**: JSON structured logging
- **Log Location**: /var/log/universal-middleware/

View logs:
```bash
# View API Gateway logs
tail -f /var/log/universal-middleware/api-gateway.log

# View WebSocket Hub logs
tail -f /var/log/universal-middleware/ws-hub.log
```

### Debugging Tools

#### Profile CPU Usage
```bash
# Generate CPU profile
curl http://localhost:8080/debug/pprof/profile > cpu.prof

# Analyze with pprof
go tool pprof cpu.prof
```

#### Memory Profiling
```bash
# Generate heap profile
curl http://localhost:8080/debug/pprof/heap > heap.prof

# Analyze memory usage
go tool pprof heap.prof
```

#### Goroutine Analysis
```bash
# View goroutine stack traces
curl http://localhost:8080/debug/pprof/goroutine
```

## Common Issues and Solutions

### Redis Connection Issues
1. **Check Redis connectivity**:
   ```bash
   redis-cli ping
   ```

2. **View Redis info**:
   ```bash
   redis-cli info
   ```

3. **Monitor Redis commands**:
   ```bash
   redis-cli monitor
   ```

### Kafka Troubleshooting
1. **List topics**:
   ```bash
   kafka-topics --bootstrap-server localhost:9092 --list
   ```

2. **View consumer groups**:
   ```bash
   kafka-consumer-groups --bootstrap-server localhost:9092 --list
   ```

3. **Check consumer lag**:
   ```bash
   kafka-consumer-groups --bootstrap-server localhost:9092 --describe --group cache-updater
   ```

### Database Issues
1. **Check connections**:
   ```bash
   psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"
   ```

2. **View slow queries**:
   ```bash
   psql -U postgres -c "SELECT * FROM pg_stat_activity WHERE state = 'active';"
   ```

## Performance Tuning

### API Gateway
- Max concurrent requests: 1000
- Read timeout: 30s
- Write timeout: 30s
- Idle timeout: 120s

### Redis
- Pool size: 100
- Min idle connections: 10
- Max retries: 3
- Retry backoff: 100ms

### Kafka
- Consumer group size: 3
- Batch size: 100
- Buffer size: 256KB
- Compression: snappy

### Database
- Max open connections: 50
- Max idle connections: 10
- Connection lifetime: 1h

## Recovery Procedures

### Service Recovery
1. Stop the service:
   ```bash
   sudo systemctl stop umw-api-gateway
   ```

2. Verify logs for errors:
   ```bash
   journalctl -u umw-api-gateway -n 100
   ```

3. Start the service:
   ```bash
   sudo systemctl start umw-api-gateway
   ```

4. Monitor logs:
   ```bash
   journalctl -u umw-api-gateway -f
   ```

### Data Recovery
1. **Redis Cache**:
   ```bash
   # Clear specific pattern
   redis-cli KEYS "pattern:*" | xargs redis-cli DEL

   # Rebuild cache
   curl -X POST http://localhost:8083/api/v1/cache/rebuild
   ```

2. **Kafka Events**:
   ```bash
   # Reset consumer group
   kafka-consumer-groups --bootstrap-server localhost:9092 \
     --group cache-updater --reset-offsets --to-earliest \
     --topic events --execute
   ```

## Security Procedures

### SSL Certificate Renewal
1. Generate new certificate:
   ```bash
   certbot renew
   ```

2. Update certificate paths in config:
   ```yaml
   server:
     tls:
       cert_file: /etc/letsencrypt/live/domain/fullchain.pem
       key_file: /etc/letsencrypt/live/domain/privkey.pem
   ```

3. Restart services:
   ```bash
   sudo systemctl restart umw-api-gateway
   ```

### Access Token Management
1. Rotate JWT keys:
   ```bash
   # Generate new key pair
   openssl genrsa -out private.pem 2048
   openssl rsa -in private.pem -pubout > public.pem
   ```

2. Update key paths in config
3. Restart auth service

## Maintenance Procedures

### Database Maintenance
1. **Vacuum Analysis**:
   ```bash
   psql -U postgres -c "VACUUM ANALYZE;"
   ```

2. **Index Maintenance**:
   ```bash
   psql -U postgres -c "REINDEX DATABASE middleware;"
   ```

### Log Rotation
- Logs are rotated daily
- Maximum 14 days retention
- Compressed after rotation

### Backup Procedures
1. **Database Backup**:
   ```bash
   pg_dump -U postgres middleware > backup.sql
   ```

2. **Configuration Backup**:
   ```bash
   tar -czf config-backup.tar.gz /etc/universal-middleware/
   ```

## Deployment Guide

### Pre-deployment Checklist
1. Run unit tests:
   ```bash
   make test
   ```

2. Run integration tests:
   ```bash
   make integration-test
   ```

3. Check dependencies:
   ```bash
   go mod verify
   ```

### Deployment Steps
1. Build services:
   ```bash
   make build
   ```

2. Deploy configuration:
   ```bash
   make deploy-config
   ```

3. Start services:
   ```bash
   make deploy
   ```

4. Verify deployment:
   ```bash
   make verify
   ```

### Rollback Procedures
1. Stop new version:
   ```bash
   make stop
   ```

2. Deploy previous version:
   ```bash
   make deploy version=previous
   ```

3. Verify rollback:
   ```bash
   make verify
   ```