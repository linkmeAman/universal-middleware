#!/bin/bash

# Set colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print timestamp with message
log() {
    echo -e "$(date '+%Y-%m-%d %H:%M:%S') $1"
}

# Function to check if a port is in use
check_port() {
    lsof -i ":$1" >/dev/null 2>&1
    return $?
}

# Function to stop a service
stop_service() {
    local service_name=$1
    local port=$2
    
    log "${YELLOW}Stopping $service_name on port $port...${NC}"
    
    if check_port "$port"; then
        # Find and kill the process using the port
        pid=$(lsof -t -i ":$port")
        if [ ! -z "$pid" ]; then
            kill -15 "$pid" 2>/dev/null
            
            # Wait for up to 5 seconds for graceful shutdown
            for i in {1..5}; do
                if ! check_port "$port"; then
                    log "${GREEN}$service_name stopped successfully${NC}"
                    return 0
                fi
                sleep 1
            done
            
            # Force kill if still running
            if check_port "$port"; then
                kill -9 "$pid" 2>/dev/null
                log "${YELLOW}$service_name force stopped${NC}"
            fi
        fi
    else
        log "${GREEN}$service_name is not running${NC}"
    fi
}

# Print header
log "${GREEN}Stopping Universal Middleware Services...${NC}"

# Stop each service
stop_service "api-gateway" "8080"
stop_service "ws-hub" "8085"
stop_service "command-service" "8082"
stop_service "processor" "8083"
stop_service "cache-updater" "8084"

# Alternative method to ensure all processes are stopped
log "${YELLOW}Ensuring no service processes remain...${NC}"
pkill -f 'bin/(api-gateway|ws-hub|command-service|processor|cache-updater)' 2>/dev/null

# Verify all services are stopped
services=("api-gateway:8080" "ws-hub:8085" "command-service:8082" "processor:8083" "cache-updater:8084")
all_stopped=true

for service in "${services[@]}"; do
    IFS=':' read -r name port <<< "$service"
    if check_port "$port"; then
        log "${RED}Warning: $name is still running on port $port${NC}"
        all_stopped=false
    fi
done

# Final status
if $all_stopped; then
    log "${GREEN}All services have been stopped successfully${NC}"
else
    log "${RED}Some services may still be running. Please check the warnings above${NC}"
    exit 1
fi

# Optional: Stop supporting services
read -p "Do you want to stop supporting services (Redis, PostgreSQL, OPA)? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log "${YELLOW}Stopping supporting services...${NC}"
    
    # Stop Redis if running
    if systemctl is-active --quiet redis-server; then
        log "Stopping Redis..."
        sudo systemctl stop redis-server
        log "${GREEN}Redis stopped${NC}"
    else
        log "${GREEN}Redis is not running${NC}"
    fi
    
    # Stop PostgreSQL if running
    if systemctl is-active --quiet postgresql; then
        log "Stopping PostgreSQL..."
        sudo systemctl stop postgresql
        log "${GREEN}PostgreSQL stopped${NC}"
    else
        log "${GREEN}PostgreSQL is not running${NC}"
    fi
    
    # Stop OPA if running
    opa_pid=$(pgrep -f "opa run --server")
    if [ ! -z "$opa_pid" ]; then
        log "Stopping OPA..."
        kill -15 "$opa_pid" 2>/dev/null
        log "${GREEN}OPA stopped${NC}"
    else
        log "${GREEN}OPA is not running${NC}"
    fi
    
    log "${GREEN}All supporting services stopped${NC}"
fi

exit 0