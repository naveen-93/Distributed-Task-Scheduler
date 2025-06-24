#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_PORT=${SERVER_PORT:-50051}  # Changed to match server's default port
REDIS_HOST=${REDIS_HOST:-localhost:6379}
DB_PATH=${DB_PATH:-./jobs.db}
WORKER_COUNT=${WORKER_COUNT:-1}

# Create log directories if they don't exist
echo -e "${BLUE}Creating log directories...${NC}"
mkdir -p log/{server,worker,redis}

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to check if a process is running
is_process_running() {
    kill -0 "$1" 2>/dev/null
}

# Function to check if port is available
is_port_available() {
    ! nc -z localhost "$1" 2>/dev/null
}

# Function to check if Redis is running
check_redis() {
    if redis-cli -h localhost -p 6379 ping > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Function to start Redis if not running
start_redis() {
    if check_redis; then
        print_status "Redis is already running"
        return 0
    fi
    
    print_status "Starting Redis server..."
    if command -v redis-server > /dev/null 2>&1; then
        redis-server --daemonize yes --logfile log/redis/redis.log --port 6379
        sleep 2
        if check_redis; then
            print_status "Redis started successfully"
            return 0
        else
            print_error "Failed to start Redis"
            return 1
        fi
    else
        print_error "Redis not found. Please install Redis or start it manually"
        return 1
    fi
}

# Array to store background process PIDs
declare -a PIDS=()
declare -a TAIL_PIDS=()

# Function to cleanup background processes on script exit
cleanup() {
    print_warning "Cleaning up processes..."
    
    # Kill all tracked processes
    for pid in "${PIDS[@]}" "${TAIL_PIDS[@]}"; do
        if is_process_running "$pid"; then
            print_status "Stopping process $pid"
            kill "$pid" 2>/dev/null
        fi
    done
    
    # Wait a bit for graceful shutdown
    sleep 2
    
    # Force kill if still running
    for pid in "${PIDS[@]}" "${TAIL_PIDS[@]}"; do
        if is_process_running "$pid"; then
            print_warning "Force killing process $pid"
            kill -9 "$pid" 2>/dev/null
        fi
    done
    
    # Clean up build artifacts if make clean exists
    if make -n clean > /dev/null 2>&1; then
        rm -rf bin
    fi
    
    print_status "Cleanup completed"
    exit 0
}

# Set up trap for cleanup on script termination
trap cleanup EXIT SIGINT SIGTERM

# Check if required commands exist
for cmd in make go redis-cli nc; do
    if ! command -v "$cmd" > /dev/null 2>&1; then
        print_error "$cmd is not installed or not in PATH"
        exit 1
    fi
done

# Start Redis
if ! start_redis; then
    print_error "Cannot proceed without Redis"
    exit 1
fi

# Build the binaries
print_status "Building binaries..."
if ! make build; then
    print_error "Build failed!"
    exit 1
fi

# Check if server port is available
if ! is_port_available "$SERVER_PORT"; then
    print_error "Port $SERVER_PORT is already in use!"
    exit 1
fi

# Run server in background with logging
print_status "Starting server on port $SERVER_PORT..."
make run-server > log/server/server.log 2>&1 &
SERVER_PID=$!
PIDS+=("$SERVER_PID")

# Wait for server to start
print_status "Waiting for server to start..."
for i in {1..30}; do  # Increased timeout to 30 seconds
    sleep 1
    if ! is_process_running "$SERVER_PID"; then
        print_error "Server failed to start! Check log/server/server.log for details"
        cat log/server/server.log
        exit 1
    fi
    
    # Check if server is responding
    if nc -z localhost "$SERVER_PORT" 2>/dev/null; then
        print_status "Server is responding on port $SERVER_PORT"
        break
    fi
    
    if [ "$i" -eq 30 ]; then  # Increased timeout check to 30 seconds
        print_error "Server didn't start responding within 30 seconds"
        cat log/server/server.log
        exit 1
    fi
done

# Start workers
for ((i=1; i<=WORKER_COUNT; i++)); do
    print_status "Starting worker $i..."
    make run-worker > log/worker/worker_$i.log 2>&1 &
    WORKER_PID=$!
    PIDS+=("$WORKER_PID")
    
    # Wait a bit for worker to start
    sleep 1
    
    # Check if worker is still running
    if ! is_process_running "$WORKER_PID"; then
        print_error "Worker $i failed to start! Check log/worker/worker_$i.log for details"
        cat log/worker/worker_$i.log
        exit 1
    fi
    
    print_status "Worker $i started successfully (PID: $WORKER_PID)"
done

# Display running services
echo
print_status "All services are running successfully!"
echo -e "${BLUE}Configuration:${NC}"
echo -e "  - Server: localhost:$SERVER_PORT"
echo -e "  - Redis: $REDIS_HOST"
echo -e "  - Database: $DB_PATH"
echo -e "  - Workers: $WORKER_COUNT"
echo
echo -e "${BLUE}Process Information:${NC}"
echo -e "  - Server PID: $SERVER_PID"
for ((i=1; i<=WORKER_COUNT; i++)); do
    echo -e "  - Worker $i PID: ${PIDS[$i]}"
done
echo
echo -e "${BLUE}Logs are available at:${NC}"
echo -e "  - Server: log/server/server.log"
for ((i=1; i<=WORKER_COUNT; i++)); do
    echo -e "  - Worker $i: log/worker/worker_$i.log"
done
echo -e "  - Redis: log/redis/redis.log"
echo
print_status "You can now submit jobs using: go run cmd/client/main.go"
print_warning "Press Ctrl+C to stop all services"
echo

# Monitor logs with colored output
print_status "Monitoring logs (press Ctrl+C to stop)..."
echo "=========================================="

# Function to tail logs with prefixes
tail_with_prefix() {
    tail -f "$2" | sed "s/^/[$1] /" &
    TAIL_PIDS+=($!)
}

# Start tailing logs
tail_with_prefix "SERVER" "log/server/server.log"
tail_with_prefix "REDIS" "log/redis/redis.log"

for ((i=1; i<=WORKER_COUNT; i++)); do
    tail_with_prefix "WORKER-$i" "log/worker/worker_$i.log"
done

# Wait for any process to exit or user interrupt
wait