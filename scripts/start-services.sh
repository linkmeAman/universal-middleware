#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create necessary directories in user's home directory
mkdir -p $HOME/.universal-middleware/log $HOME/.universal-middleware/run
PID_DIR="$HOME/.universal-middleware/run"
LOG_DIR="$HOME/.universal-middleware/log"

# Function to log messages
log() {
    echo -e "${2:-$GREEN}$(date '+%Y-%m-%d %H:%M:%S') $1${NC}"
}

# Check dependencies
check_dependencies() {
    log "Checking dependencies..." "$YELLOW"
    
    # Run database migrations
    PGPASSWORD=postgres psql -h localhost -U postgres middleware -f migrations/000006_create_outbox_table.up.sql || {
        log "Failed to run database migrations" "$RED"
        return 1
    }
    
    # Check Redis
    if ! redis-cli ping >/dev/null 2>&1; then
        log "Redis is not running. Starting Redis..." "$YELLOW"
        sudo systemctl start redis-server
        sleep 2
        if ! redis-cli ping >/dev/null 2>&1; then
            log "Failed to start Redis" "$RED"
            return 1
        fi
    fi
    
    # Check PostgreSQL
    if ! pg_isready >/dev/null 2>&1; then
        log "PostgreSQL is not running. Starting PostgreSQL..." "$YELLOW"
        sudo systemctl start postgresql
        sleep 2
        if ! pg_isready >/dev/null 2>&1; then
            log "Failed to start PostgreSQL" "$RED"
            return 1
        fi
    fi
    
    # Check Kafka
    if ! ss -tunlp 2>/dev/null | grep -q ":9092"; then
        log "Kafka is not running. Starting Kafka..." "$YELLOW"
        /opt/kafka/bin/zookeeper-server-start.sh -daemon /etc/kafka/zookeeper.properties
        sleep 5
        /opt/kafka/bin/kafka-server-start.sh -daemon /etc/kafka/server.properties
        sleep 5
        if ! ss -tunlp 2>/dev/null | grep -q ":9092"; then
            log "Failed to start Kafka" "$RED"
            return 1
        fi
    fi

    # Create Kafka topics if they don't exist
    for topic in events commands cache; do
        /opt/kafka/bin/kafka-topics.sh --create --if-not-exists \
            --bootstrap-server localhost:9092 \
            --topic $topic \
            --partitions 1 \
            --replication-factor 1 >/dev/null 2>&1
    done

    return 0
}

# Function to start a service
start_service() {
    local service=$1
    local port=$2
    log "Starting $service on port $port..." "$YELLOW"
    
    # Check if service binary exists
    if [ ! -f "./bin/$service" ]; then
        log "Service binary not found: ./bin/$service" "$RED"
        return 1
    fi

    # Kill existing instance if running
    if pgrep -f "bin/$service" > /dev/null; then
        log "Stopping existing $service instance..." "$YELLOW"
        pkill -f "bin/$service"
        sleep 2
    fi

    # Start the service
    ./bin/$service > $LOG_DIR/$service.log 2>&1 &
    local pid=$!
    
    # Wait for service to start
    sleep 2
    
    if kill -0 $pid 2>/dev/null; then
        log "$service started successfully (PID: $pid)" "$GREEN"
        echo $pid > $PID_DIR/$service.pid
        return 0
    else
        log "Failed to start $service" "$RED"
        log "Last few lines of log:" "$YELLOW"
        tail -n 5 $LOG_DIR/$service.log
        return 1
    fi
}

# Run dependency checks
if ! check_dependencies; then
    log "Failed to start required dependencies" "$RED"
    exit 1
fi

# Build services
log "Building services..." "$YELLOW"
if ! make build; then
    log "Failed to build services" "$RED"
    exit 1
fi

# Start services in order with their respective ports
start_service "api-gateway" 8080 || exit 1
start_service "ws-hub" 8085 || exit 1
start_service "command-service" 8082 || exit 1
start_service "processor" 8083 || exit 1
start_service "cache-updater" 8084 || exit 1

# Final health check function
check_service_health() {
    local service=$1
    local port=$2
    local max_retries=5
    local retry_delay=2

    # Use the correct ports for each service
    local actual_port=$port
    case $service in
        "command-service") actual_port=8082 ;;
        "processor") actual_port=8083 ;;
        "cache-updater") actual_port=8084 ;;
    esac

    for ((i=1; i<=max_retries; i++)); do
        log "Checking $service health (attempt $i of $max_retries)..." "$YELLOW"
        
        # Check if process is running
        local pid
        if [ -f "$PID_DIR/$service.pid" ]; then
            pid=$(cat "$PID_DIR/$service.pid")
        else
            ps aux | grep "bin/$service" | grep -v grep | awk '{print $2}' | head -n 1
        fi

        if [ ! -z "$pid" ] && kill -0 $pid 2>/dev/null; then
            # Process is running, try health check
            response=$(curl -s "http://localhost:$actual_port/health")
            curl_status=$?
            if [ $curl_status -eq 0 ]; then
                if [ "$response" = "healthy" ]; then
                    log "$service is healthy" "$GREEN"
                    return 0
                fi
                status=$(echo "$response" | jq -r '.status' 2>/dev/null)
                # If jq returns null or an error, try simple pattern matching
                if [ -z "$status" ] || [ "$status" = "null" ]; then
                    if [[ "$response" =~ "\"status\":\"healthy\"" ]]; then
                        status="healthy"
                    fi
                fi
                if [ "$status" = "healthy" ]; then
                    log "$service is healthy" "$GREEN"
                    return 0
                else
                    log "$service is not healthy: $response" "$YELLOW"
                    log "Last few lines of log:" "$YELLOW"
                    tail -n 10 $LOG_DIR/$service.log
                fi
            else
                log "Failed to get health status from $service at port $actual_port (curl exit code: $curl_status)" "$RED"
                log "Curl response: $response" "$RED"
                log "Last few lines of log:" "$YELLOW"
                tail -n 10 /var/log/universal-middleware/$service.log
            fi
        else
            log "$service is not running (no valid PID)" "$RED"
            if [ -f "/var/log/universal-middleware/$service.log" ]; then
                log "Last few lines of log:" "$YELLOW"
                tail -n 10 $LOG_DIR/$service.log
            else
                log "No log file found at $LOG_DIR/$service.log" "$RED"
            fi
        fi

        if [ $i -lt $max_retries ]; then
            log "Retrying in $retry_delay seconds..." "$YELLOW"
            sleep $retry_delay
        fi
    done

    return 1
}

# Final status check
log "\nChecking final service status..." "$YELLOW"
services=("api-gateway:8080" "ws-hub:8085" "command-service:8082" "processor:8083" "cache-updater:8084")

failed_services=()
for service_port in "${services[@]}"; do
    IFS=':' read -r service port <<< "$service_port"
    if ! check_service_health "$service" "$port"; then
        failed_services+=("$service")
    fi
done

if [ ${#failed_services[@]} -eq 0 ]; then
    log "\nAll services are healthy!" "$GREEN"
    exit 0
else
    log "\nSome services failed health checks:" "$RED"
    for service in "${failed_services[@]}"; do
        log "- $service" "$RED"
    done
    exit 1
fi
