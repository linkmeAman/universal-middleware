#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to log messages
log() {
    echo -e "${2:-$GREEN}$(date '+%Y-%m-%d %H:%M:%S') $1${NC}"
}

# Function to check if a command exists
check_command() {
    command -v "$1" >/dev/null 2>&1
}

# Check if script is run as root
if [ "$EUID" -ne 0 ]; then 
    log "Please run as root or with sudo" "$RED"
    exit 1
fi

# Check system resources
log "Checking system requirements..." "$YELLOW"
TOTAL_MEM=$(free -g | awk '/^Mem:/{print $2}')
TOTAL_DISK=$(df -BG / | awk 'NR==2 {print $4}' | sed 's/G//')
CPU_CORES=$(nproc)

if [ "$TOTAL_MEM" -lt 4 ]; then
    log "Warning: Less than 4GB RAM available. 8GB recommended." "$YELLOW"
fi

if [ "$TOTAL_DISK" -lt 20 ]; then
    log "Warning: Less than 20GB disk space available." "$YELLOW"
fi

if [ "$CPU_CORES" -lt 2 ]; then
    log "Warning: Less than 2 CPU cores available. 4 cores recommended." "$YELLOW"
fi

# Install Go
log "Installing Go..." "$YELLOW"
if ! check_command go; then
    wget https://go.dev/dl/go1.21.3.linux-amd64.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf go1.21.3.linux-amd64.tar.gz
    rm go1.21.3.linux-amd64.tar.gz
    
    # Add Go to PATH for all users
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
    echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> /etc/profile.d/go.sh
    source /etc/profile.d/go.sh
fi

# Install PostgreSQL
log "Installing PostgreSQL..." "$YELLOW"
apt-get update
apt-get install -y postgresql postgresql-contrib

# Start PostgreSQL
systemctl enable postgresql
systemctl start postgresql

# Create database and set password
sudo -u postgres psql -c "CREATE DATABASE middleware;" || true
sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD 'postgres';"

# Install Redis
log "Installing Redis..." "$YELLOW"
apt-get install -y redis-server

# Configure Redis
systemctl enable redis-server
systemctl start redis-server

# Install Protocol Buffers Compiler
log "Installing protoc..." "$YELLOW"
apt-get install -y protobuf-compiler

# Install Go protobuf plugins
log "Installing Go protobuf plugins..." "$YELLOW"
sudo -u $SUDO_USER go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
sudo -u $SUDO_USER go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Install OPA
log "Installing Open Policy Agent..." "$YELLOW"
curl -L -o opa https://openpolicyagent.org/downloads/v0.42.0/opa_linux_amd64
chmod 755 opa
mv opa /usr/local/bin/

# Create log directory
log "Creating log directories..." "$YELLOW"
mkdir -p /var/log/universal-middleware
chown -R $SUDO_USER:$SUDO_USER /var/log/universal-middleware

# Setup environment variables
log "Setting up environment variables..." "$YELLOW"
cat << EOF > /etc/profile.d/universal-middleware.sh
export MIDDLEWARE_ENV=development
export MIDDLEWARE_CONFIG_PATH=/home/$SUDO_USER/universal-middleware/config
export MIDDLEWARE_LOG_LEVEL=debug
export MIDDLEWARE_DB_HOST=localhost
export MIDDLEWARE_DB_PORT=5432
export MIDDLEWARE_DB_NAME=middleware
export MIDDLEWARE_DB_USER=postgres
export MIDDLEWARE_DB_PASSWORD=postgres
export MIDDLEWARE_REDIS_HOST=localhost
export MIDDLEWARE_REDIS_PORT=6379
EOF

# Make the environment file executable
chmod +x /etc/profile.d/universal-middleware.sh

# Verify installations
log "Verifying installations..." "$YELLOW"

# Check Go
if go version > /dev/null 2>&1; then
    log "✓ Go installed successfully" "$GREEN"
else
    log "✗ Go installation failed" "$RED"
fi

# Check PostgreSQL
if pg_isready > /dev/null 2>&1; then
    log "✓ PostgreSQL installed and running" "$GREEN"
else
    log "✗ PostgreSQL installation failed" "$RED"
fi

# Check Redis
if redis-cli ping > /dev/null 2>&1; then
    log "✓ Redis installed and running" "$GREEN"
else
    log "✗ Redis installation failed" "$RED"
fi

# Check OPA
if opa version > /dev/null 2>&1; then
    log "✓ OPA installed successfully" "$GREEN"
else
    log "✗ OPA installation failed" "$RED"
fi

log "Setup complete! Please run 'source /etc/profile.d/universal-middleware.sh' to load the environment variables" "$GREEN"
log "Next steps:" "$YELLOW"
log "1. cd /home/$SUDO_USER/universal-middleware" "$YELLOW"
log "2. make deps" "$YELLOW"
log "3. make build" "$YELLOW"
log "4. ./scripts/start-services.sh" "$YELLOW"