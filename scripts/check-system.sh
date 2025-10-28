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

# Function to check command existence
check_command() {
    if ! command -v $1 &> /dev/null; then
        log "${RED}❌ $1 is not installed${NC}"
        return 1
    else
        log "${GREEN}✓ $1 is installed: $(command -v $1)${NC}"
        return 0
    fi
}

# Function to check minimum version
check_version() {
    local command=$1
    local min_version=$2
    local version_cmd=$3
    
    local version=$($version_cmd)
    if [[ "$version" == *"$min_version"* ]] || [[ "$version" > "$min_version" ]]; then
        log "${GREEN}✓ $command version $version meets minimum requirement ($min_version)${NC}"
        return 0
    else
        log "${RED}❌ $command version $version does not meet minimum requirement ($min_version)${NC}"
        return 1
    fi
}

# Function to check system resources
check_resources() {
    log "\n${YELLOW}Checking System Resources...${NC}"
    
    # Check CPU cores
    local cpu_cores=$(nproc)
    if [ "$cpu_cores" -lt 2 ]; then
        log "${RED}❌ Insufficient CPU cores: $cpu_cores (minimum 2 required)${NC}"
        return 1
    else
        log "${GREEN}✓ CPU cores: $cpu_cores${NC}"
    fi
    
    # Check RAM
    local total_ram=$(free -m | awk '/^Mem:/{print $2}')
    if [ "$total_ram" -lt 4096 ]; then
        log "${RED}❌ Insufficient RAM: $total_ram MB (minimum 4096 MB required)${NC}"
        return 1
    else
        log "${GREEN}✓ RAM: $total_ram MB${NC}"
    fi
    
    # Check disk space
    local free_space=$(df -m . | awk 'NR==2 {print $4}')
    if [ "$free_space" -lt 20480 ]; then
        log "${RED}❌ Insufficient disk space: $free_space MB (minimum 20 GB required)${NC}"
        return 1
    else
        log "${GREEN}✓ Free disk space: $free_space MB${NC}"
    fi
    
    return 0
}

# Function to check system packages
check_packages() {
    log "\n${YELLOW}Checking Required Packages...${NC}"
    
    local missing_packages=()
    local required_packages=(
        "make"
        "git"
        "curl"
        "wget"
        "lsof"
        "jq"
    )
    
    for package in "${required_packages[@]}"; do
        if ! dpkg -l | grep -q "^ii  $package "; then
            missing_packages+=("$package")
        else
            log "${GREEN}✓ $package is installed${NC}"
        fi
    done
    
    if [ ${#missing_packages[@]} -ne 0 ]; then
        log "${RED}❌ Missing required packages: ${missing_packages[*]}${NC}"
        log "${YELLOW}Run: sudo apt-get install ${missing_packages[*]}${NC}"
        return 1
    fi
    
    return 0
}

# Function to check required services
check_services() {
    log "\n${YELLOW}Checking Required Services...${NC}"
    
    # Check PostgreSQL installation and version
    if ! check_command "psql"; then
        log "${YELLOW}To install PostgreSQL:${NC}"
        echo "sudo apt-get update"
        echo "sudo apt-get install -y postgresql postgresql-contrib"
        return 1
    fi
    check_version "PostgreSQL" "15" "psql --version"
    
    # Check Redis installation
    if ! check_command "redis-cli"; then
        log "${YELLOW}To install Redis:${NC}"
        echo "sudo apt-get update"
        echo "sudo apt-get install -y redis-server"
        return 1
    fi
    
    # Check Go installation and version
    if ! check_command "go"; then
        log "${YELLOW}To install Go:${NC}"
        echo "wget https://go.dev/dl/go1.21.3.linux-amd64.tar.gz"
        echo "sudo rm -rf /usr/local/go"
        echo "sudo tar -C /usr/local -xzf go1.21.3.linux-amd64.tar.gz"
        echo "export PATH=\$PATH:/usr/local/go/bin"
        return 1
    fi
    check_version "Go" "1.21" "go version"
    
    # Check protoc installation
    if ! check_command "protoc"; then
        log "${YELLOW}To install protoc:${NC}"
        echo "sudo apt-get install -y protobuf-compiler"
        return 1
    fi
    
    # Check protoc-gen-go installation
    if ! check_command "protoc-gen-go"; then
        log "${YELLOW}To install protoc-gen-go:${NC}"
        echo "go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        echo "go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
        return 1
    fi
    
    # Check OPA installation
    if ! check_command "opa"; then
        log "${YELLOW}To install OPA:${NC}"
        echo "curl -L -o opa https://openpolicyagent.org/downloads/v0.42.0/opa_linux_amd64"
        echo "chmod 755 opa"
        echo "sudo mv opa /usr/local/bin/"
        return 1
    fi
    
    return 0
}

# Function to check environment setup
check_environment() {
    log "\n${YELLOW}Checking Environment Setup...${NC}"
    
    # Check GOPATH
    if [ -z "$GOPATH" ]; then
        log "${RED}❌ GOPATH is not set${NC}"
        return 1
    else
        log "${GREEN}✓ GOPATH is set to: $GOPATH${NC}"
    fi
    
    # Check if Go bin is in PATH
    if [[ ":$PATH:" != *":$GOPATH/bin:"* ]]; then
        log "${RED}❌ GOPATH/bin is not in PATH${NC}"
        log "${YELLOW}Add to ~/.bashrc:${NC}"
        echo "export PATH=\$PATH:\$(go env GOPATH)/bin"
        return 1
    else
        log "${GREEN}✓ GOPATH/bin is in PATH${NC}"
    fi
    
    # Check required environment variables
    local required_vars=(
        "MIDDLEWARE_ENV"
        "MIDDLEWARE_CONFIG_PATH"
        "MIDDLEWARE_LOG_LEVEL"
        "MIDDLEWARE_DB_HOST"
        "MIDDLEWARE_DB_PORT"
        "MIDDLEWARE_DB_NAME"
        "MIDDLEWARE_DB_USER"
        "MIDDLEWARE_DB_PASSWORD"
        "MIDDLEWARE_REDIS_HOST"
        "MIDDLEWARE_REDIS_PORT"
    )
    
    local missing_vars=()
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            missing_vars+=("$var")
        else
            log "${GREEN}✓ $var is set${NC}"
        fi
    done
    
    if [ ${#missing_vars[@]} -ne 0 ]; then
        log "${RED}❌ Missing required environment variables: ${missing_vars[*]}${NC}"
        return 1
    fi
    
    return 0
}

# Function to check ports availability
check_ports() {
    log "\n${YELLOW}Checking Port Availability...${NC}"
    
    local ports=(
        8080  # API Gateway
        8085  # WebSocket Hub
        8082  # Command Service
        8083  # Processor
        8084  # Cache Updater
        6379  # Redis
        5432  # PostgreSQL
        8181  # OPA
    )
    
    local busy_ports=()
    for port in "${ports[@]}"; do
        if lsof -i ":$port" >/dev/null 2>&1; then
            busy_ports+=("$port")
        else
            log "${GREEN}✓ Port $port is available${NC}"
        fi
    done
    
    if [ ${#busy_ports[@]} -ne 0 ]; then
        log "${RED}❌ Following ports are in use: ${busy_ports[*]}${NC}"
        return 1
    fi
    
    return 0
}

# Main execution
log "${YELLOW}Starting System Check for Universal Middleware...${NC}"

# Array to track failures
failures=()

# Run all checks
check_resources || failures+=("System Resources")
check_packages || failures+=("Required Packages")
check_services || failures+=("Required Services")
check_environment || failures+=("Environment Setup")
check_ports || failures+=("Port Availability")

# Final report
if [ ${#failures[@]} -eq 0 ]; then
    log "\n${GREEN}✓ All checks passed! System is ready to run Universal Middleware${NC}"
    exit 0
else
    log "\n${RED}❌ System check failed. Please fix the following issues:${NC}"
    for failure in "${failures[@]}"; do
        log "${RED}- $failure${NC}"
    done
    exit 1
fi