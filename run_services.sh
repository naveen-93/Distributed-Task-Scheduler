#!/bin/zsh

# Function to cleanup background processes on script exit
cleanup() {
    echo "Cleaning up processes..."
    jobs -p | xargs -r kill
    make clean
    exit 0
}

# Set up trap for cleanup on script termination
trap cleanup EXIT SIGINT SIGTERM

# Build the binaries
echo "Building binaries..."
make build

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

# Run server in background
echo "Starting server..."
make run-server &
SERVER_PID=$!

# Wait a bit for server to start
sleep 2

# Check if server is still running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "Server failed to start!"
    exit 1
fi

# Run worker in background
echo "Starting worker..."
make run-worker &
WORKER_PID=$!

# Wait a bit for worker to start
sleep 2

# Check if worker is still running
if ! kill -0 $WORKER_PID 2>/dev/null; then
    echo "Worker failed to start!"
    exit 1
fi

echo "All services are running!"
echo "Press Ctrl+C to stop all services"

# Wait for any process to exit
wait 