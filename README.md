# Distributed Task Scheduler

A distributed task scheduling system built in Go that consists of three main components: a server, workers, and a client. The system uses Redis for job queue management and gRPC for communication between components.

## System Architecture

- **Server**: Central coordinator that manages job scheduling and distribution
- **Worker**: Executes the assigned tasks and reports back to the server
- **Client**: Submits jobs to the server
- **Redis**: Used as a message queue for job distribution
- **SQLite**: Stores job information and execution history

## Prerequisites

- Go 1.x or higher
- Redis server
- Make build tool
- SQLite

## Project Structure

```
Distributed-Task-Scheduler/
├── cmd/                    # Main applications
│   ├── client/            # Client executable
│   ├── server/            # Server executable
│   └── worker/            # Worker executable
├── internal/              # Internal packages
│   ├── client/           # Client implementation
│   ├── db/               # Database operations
│   ├── queue/            # Redis queue management
│   ├── server/           # Server implementation
│   └── worker/           # Worker implementation
├── proto/                 # Protocol Buffers definitions
├── Makefile              # Build and run commands
├── protogen.sh           # Protocol Buffers generation script
└── run_services.sh       # Script to run server and worker
```

## Building the Project

Use the Makefile to build all components:

```bash
make build
```

This will create binaries in the `bin/` directory.

## Running the Services

### Option 1: Using Individual Commands

1. Start the server:
```bash
make run-server
```

2. Start a worker:
```bash
make run-worker
```

3. Run a client command:
```bash
make run-client CMD='your command'
```

### Option 2: Using the Convenience Script

Script that runs both the server and worker:

```bash
bash run_services.sh
```

This script will:
- Build all necessary binaries
- Start the server and worker processes
- Handle graceful shutdown on Ctrl+C
- Clean up processes automatically

## Available Make Commands

- `make build`: Build all binaries
- `make run-server`: Run the server
- `make run-worker`: Run the worker
- `make run-client`: Run the client (optional: CMD='your command')
- `make clean`: Remove build artifacts
- `make all`: Clean and rebuild all
- `make help`: Show available commands

## Development

### Regenerating Protocol Buffers

If you modify the protocol buffer definitions, regenerate the Go code:

```bash
bash protogen.sh
```

### Database Schema

The SQLite database schema is located in `internal/db/schema.sql`.

## Architecture Details

### Server
- Manages job scheduling and distribution
- Maintains job status and history
- Coordinates with workers through gRPC
- Uses Redis for job queue management

### Worker
- Connects to the server via gRPC
- Pulls jobs from Redis queue
- Executes assigned tasks
- Reports execution results back to the server

### Client
- Submits jobs to the server
- Can query job status and history
- Communicates with server via gRPC

## Error Handling

- The system implements robust error handling and recovery
- Failed jobs are automatically retried based on configuration
- Workers can reconnect automatically if connection is lost
- All operations are logged for debugging purposes


