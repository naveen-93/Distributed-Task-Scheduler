.PHONY: build clean run-server run-worker run-client all

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

# Run client (with optional command flag)
run-client: $(CLIENT_BINARY)
	./$(CLIENT_BINARY) $(if $(CMD),-cmd='$(CMD)')

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR) log

# Default target
all: clean build

# Helper target to show usage
help:
	@echo "Available targets:"
	@echo "  make build      - Build all binaries"
	@echo "  make run-server - Run the server"
	@echo "  make run-worker - Run the worker"
	@echo "  make run-client - Run the client (optional: CMD='your command')"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make all        - Clean and build all"
	@echo "  make help       - Show this help"
	@echo ""
	@echo "Example usage:"
	@echo "  make run-client CMD='ls -la'" 