.PHONY: build clean run-server run-worker run-client run-client-json run-client-concurrent run-client-file all

# Binary names and paths
BINARY_DIR = bin
SERVER_BINARY = $(BINARY_DIR)/server
WORKER_BINARY = $(BINARY_DIR)/worker
CLIENT_BINARY = $(BINARY_DIR)/client

# Build flags
GO = go
GOFLAGS = -v

# Build all binaries
build: $(BINARY_DIR) $(SERVER_BINARY) $(WORKER_BINARY) $(CLIENT_BINARY)

# Create bin directory
$(BINARY_DIR):
	mkdir -p $(BINARY_DIR)

# Build server
$(SERVER_BINARY):
	$(GO) build $(GOFLAGS) -o $(SERVER_BINARY) ./cmd/server

# Build worker
$(WORKER_BINARY):
	$(GO) build $(GOFLAGS) -o $(WORKER_BINARY) ./cmd/worker

# Build client
$(CLIENT_BINARY):
	$(GO) build $(GOFLAGS) -o $(CLIENT_BINARY) ./cmd/client

# Run server
run-server: $(SERVER_BINARY)
	./$(SERVER_BINARY)

# Run worker
run-worker: $(WORKER_BINARY)
	./$(WORKER_BINARY)

# Run client (with optional JSON file and concurrent flag)
run-client: $(CLIENT_BINARY)
	./$(CLIENT_BINARY) $(if $(FILE),-file='$(FILE)') $(if $(CONCURRENT),-concurrent)

# Run client with default jobs.json file
run-client-json: $(CLIENT_BINARY)
	./$(CLIENT_BINARY) -file=jobs.json

# Run client with jobs concurrently
run-client-concurrent: $(CLIENT_BINARY)
	./$(CLIENT_BINARY) -file=jobs.json -concurrent

# Run client with custom JSON file
run-client-file: $(CLIENT_BINARY)
	@if [ -z "$(FILE)" ]; then echo "Usage: make run-client-file FILE=your-jobs.json"; exit 1; fi
	./$(CLIENT_BINARY) -file=$(FILE)

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR) log

# Default target
all: clean build

# Helper target to show usage
help:
	@echo "Available targets:"
	@echo "  make build               - Build all binaries"
	@echo "  make run-server          - Run the server"
	@echo "  make run-worker          - Run the worker"
	@echo "  make run-client          - Run the client with options (FILE='file.json' CONCURRENT=true)"
	@echo "  make run-client-json     - Run the client with default jobs.json"
	@echo "  make run-client-concurrent - Run jobs concurrently from jobs.json"
	@echo "  make run-client-file     - Run with custom file (make run-client-file FILE=your-jobs.json)"
	@echo "  make clean               - Remove build artifacts"
	@echo "  make all                 - Clean and build all"
	@echo "  make help                - Show this help"
	@echo ""
	@echo "Example usage:"
	@echo "  make run-client-json                    # Use default jobs.json"
	@echo "  make run-client FILE=my-jobs.json       # Use custom JSON file"
	@echo "  make run-client CONCURRENT=true         # Run jobs concurrently"
	@echo "  make run-client-file FILE=custom.json   # Use specific file" 