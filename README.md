# Distributed Task Scheduler

A high-performance distributed task scheduler built with Go, featuring gRPC communication, Redis queue management, and SQLite job persistence.

## 🚀 Features

- **Distributed Architecture**: Scalable server-worker model
- **gRPC Communication**: Fast, type-safe client-server communication
- **Redis Queue**: Reliable FIFO job queue with Redis
- **SQLite Persistence**: Durable job storage and status tracking
- **Real-time Status**: Live job status monitoring
- **Concurrent Processing**: Multiple workers can process jobs simultaneously
- **Command Execution**: Execute any shell command as a job
- **Comprehensive Logging**: Detailed logging for debugging and monitoring

## 🏗️ Architecture

```
┌─────────┐    gRPC     ┌─────────┐    Redis    ┌─────────┐
│ Client  │ ────────────▶ Server  │ ────────────▶ Worker  │
└─────────┘             └─────────┘             └─────────┘
                             │                       │
                             ▼                       ▼
                        ┌─────────┐             ┌─────────┐
                        │ SQLite  │             │ Command │
                        │   DB    │             │ Executor│
                        └─────────┘             └─────────┘
```

## 📋 Prerequisites

- **Go** 1.19 or higher
- **Redis** server
- **Protocol Buffers** compiler (for development)

### Installing Dependencies

**macOS:**
```bash
brew install go redis protobuf
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install golang-go redis-server protobuf-compiler
```

## 🛠️ Installation

1. **Clone the repository:**
```bash
git clone https://github.com/naveen-93/Distributed-Task-Scheduler.git
cd distributed-task-scheduler
```

2. **Install Go dependencies:**
```bash
go mod download
```

3. **Build the binaries:**
```bash
make build
```

## 🚀 Quick Start

### 1. Start Redis (if not running)
```bash
redis-server --daemonize yes
```

### 2. Start the Server
```bash
make run-server
```

### 3. Start Worker(s)
```bash
# Terminal 2
make run-worker
```

### 4. Submit Jobs
```bash
# Terminal 3 - Submit default job
make run-client

# Submit custom job
make run-client CMD='echo "Hello, World!"'

# Submit heavy computation
make run-client CMD='python3 -c "import time; time.sleep(5); print(\"Task completed!\")"'
```

## 📚 Usage Examples

### Basic Job Submission
```bash
# Simple command
make run-client CMD='ls -la'

# Python computation
make run-client CMD='python3 -c "print(sum(range(1000)))"'

# File operations
make run-client CMD='du -sh /tmp'
```

### Heavy Tasks for Testing
```bash
# CPU-intensive: Fibonacci calculation
make run-client CMD='python3 -c "import time; start=time.time(); fib=lambda n: n if n<=1 else fib(n-1)+fib(n-2); result=fib(35); print(f\"fib(35)={result}, took {time.time()-start:.2f}s\")"'

# I/O-intensive: File operations
make run-client CMD='dd if=/dev/zero of=/tmp/test_file bs=1M count=100 && rm /tmp/test_file'

# Long-running: Progress simulation
make run-client CMD='python3 -c "import time; [print(f\"Progress: {i}/10\") or time.sleep(1) for i in range(1,11)]"'
```

### Programmatic Usage

You can also submit jobs programmatically using the Go client:

```go
package main

import (
    "context"
    "log"
    pb "distributed-task-scheduler/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewJobServiceClient(conn)
    
    // Submit job
    resp, err := client.SubmitJob(context.Background(), &pb.Job{
        Command: "echo 'Hello from Go client!'",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Job submitted: %s", resp.JobId)
}
```

## 🔧 Configuration

### Server Configuration
The server runs on `localhost:50051` by default. To change:

```go
// cmd/server/main.go
const serverAddr = "localhost:8080"  // Change port
```

### Worker Configuration
Workers connect to:
- **Server**: `localhost:50051`
- **Redis**: `localhost:6379`
- **Database**: `./jobs.db`

### Redis Configuration
Default Redis settings work out of the box. For custom Redis:

```go
// internal/queue/queue.go
redis.NewClient(&redis.Options{
    Addr:     "localhost:6380",  // Custom port
    Password: "your-password",   // Add password
    DB:       1,                 // Different database
})
```

## 🧪 Testing

### Run Tests
```bash
go test ./...
```

### Manual Testing
```bash
# Test with multiple workers
make run-worker  # Terminal 1
make run-worker  # Terminal 2
make run-worker  # Terminal 3

# Submit multiple jobs
for i in {1..10}; do make run-client CMD="echo 'Job $i'"; done
```

### Performance Testing
```bash
# Submit CPU-intensive jobs
for i in {1..5}; do 
  make run-client CMD="python3 -c 'import time; time.sleep(2); print(\"Job $i done\")'" &
done
```

## 📊 Monitoring

### Job Status
Jobs progress through these states:
- `PENDING` → `RUNNING` → `SUCCEEDED`/`FAILED`

### Logging
- **Server logs**: Job submissions, queue operations
- **Worker logs**: Job processing, execution results
- **Redis logs**: Queue operations (if enabled)

### Database Inspection
```bash
sqlite3 jobs.db "SELECT id, status, command, created_at FROM jobs ORDER BY created_at DESC LIMIT 10;"
```

## 🔍 Troubleshooting

### Common Issues

**Jobs stuck in PENDING:**
```bash
# Check if Redis is running
redis-cli ping

# Check for multiple Redis instances
ps aux | grep redis

# Kill conflicting Redis processes
pkill redis-server
redis-server --daemonize yes
```

**Worker not processing jobs:**
```bash
# Check worker logs
tail -f log/worker/worker_1.log

# Verify Redis connection
redis-cli LLEN pending_jobs
```

**Connection refused errors:**
```bash
# Check if server is running
netstat -ln | grep 50051

# Restart server
make run-server
```

### Debug Mode
For detailed debugging, check the logs or add debug prints:

```go
// Add to worker.go for more verbose logging
log.Printf("Worker attempting to pop job...")
```


### Building from Source

```bash
# Generate protobuf code (if modified)
protoc --go_out=. --go-grpc_out=. proto/scheduler.proto

# Build all components
make build

# Clean and rebuild
make clean && make build
```





