# Distributed Task Scheduler

A high-performance distributed task scheduler built with Go, featuring gRPC communication, Redis queue management, and PostgreSQL job persistence.

## 🚀 Features

- **Distributed Architecture**: Scalable server-worker model
- **gRPC Communication**: Fast, type-safe client-server communication
- **Redis Queue**: Reliable FIFO job queue with Redis
- **PostgreSQL Persistence**: Durable centralized job storage and status tracking
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
                        │ Postgres│             │ Command │
                        │   DB    │             │ Executor│
                        └─────────┘             └─────────┘
```

## 📋 Prerequisites

- **Go** 1.19 or higher
- **Redis** server
- **PostgreSQL** 12+ (16 recommended)
- **Protocol Buffers** compiler (for development)
- Optional for HA: **etcd** (3 or 5 node cluster in production; single node ok for local)

### Installing Dependencies

**macOS:**
```bash
brew install go redis postgresql@16 etcd protobuf
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

### 1) Create .env
Minimal example (adjust as needed):
```bash
cat > .env <<'ENV'
DATABASE_URL=postgres://<user>:<pass>@localhost:5432/scheduler?sslmode=disable
REDIS_ADDR=localhost:6379
# Optional HA via etcd
ETCD_ENDPOINTS=localhost:2379
ELECTION_NAMESPACE=/scheduler/v1
ELECTION_KEY=leader
LEASE_TTL=10s
# Optional: pgx pool tuning
PG_MAX_CONNS=50
PG_MIN_CONNS=5
PG_MAX_CONN_LIFETIME=30m
ENV
```

### 2) Start everything
```bash
make start SERVER_COUNT=3 WORKER_COUNT=2 WEBUI_ADDR=:8080
```
- Starts Redis/Postgres/etcd via Homebrew if available, then servers, workers, and the Web UI.
- Writes a `.servers` file with the running server addresses.

### 3) Submit jobs (CLI)
```bash
# Use all servers discovered by make start
make run-client-servers FILE=jobs.json

# Or target a specific server only
./bin/client -file=jobs.json -servers=localhost:50052

# Or submit to one and poll from others
./bin/client -file=jobs.json \
  -submit-server=localhost:50052 \
  -status-servers=localhost:50051,localhost:50053
```

### 4) Web UI
```bash
make run-webui         # or launched by make start on :8080
open http://localhost:8080
```
- Pick a submit server, a status server, and enter the command to execute.

### 5) Stop everything
```bash
make stop
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
- The server listens on port from `SERVER_PORT` (default: 50051).
- Leader election (optional HA) uses etcd via `ETCD_ENDPOINTS`, `ELECTION_NAMESPACE`, `ELECTION_KEY`, `LEASE_TTL`.

### Worker Configuration
Workers connect to shared infra:
- **Redis**: `REDIS_ADDR` (default `localhost:6379`)
- **Database**: `DATABASE_URL` (PostgreSQL DSN)

### Database Configuration
### High Availability (Leader Election)
- Set these envs to enable leader election (etcd-backed):
  - `ETCD_ENDPOINTS=localhost:2379` (comma-separated for multiple)
  - `ELECTION_NAMESPACE=/scheduler/v1`
  - `ELECTION_KEY=leader`
  - `LEASE_TTL=10s`
- Only the elected leader runs maintenance loops (e.g., stale RUNNING job cleanup).
- All scheduler instances are otherwise stateless and can serve gRPC calls.
- Set via `.env` (`DATABASE_URL=postgres://user:pass@localhost:5432/scheduler?sslmode=disable`)
- Optional pooling envs (in `.env`):
  - `PG_MAX_CONNS` (e.g., `50`)
  - `PG_MIN_CONNS` (e.g., `5`)
  - `PG_MAX_CONN_LIFETIME` (e.g., `30m`)

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
- **Server logs**: Job submissions, queue operations, leader election
- **Worker logs**: Job processing, execution results
- **Redis logs**: Queue operations (if enabled)

### Database Inspection
```bash
psql "$DATABASE_URL" -c "SELECT id, status, command, to_timestamp(created_at) AS created FROM tasks ORDER BY created_at DESC LIMIT 10;"
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
# Check if servers are running
netstat -ln | egrep '5005[1-9]'

# Start or restart services
make start
```

## 🧰 Make Targets (Cheatsheet)

- **Build**: `make build`
- **Start all**: `make start SERVER_COUNT=3 WORKER_COUNT=2 WEBUI_ADDR=:8080`
- **Stop all**: `make stop`
- **Run N servers**: `make run-servers SERVER_COUNT=3 BASE_PORT=50051`
- **Stop servers**: `make stop-servers`
- **Run a worker**: `make run-worker`
- **Run client across discovered servers**: `make run-client-servers FILE=jobs.json`
- **Run Web UI**: `make run-webui` (default `:8080`)

## 🧑‍💻 Client Options

- Round-robin across servers:
  - Env: `SERVERS=host1:50051,host2:50052,host3:50053`
  - Flag: `-servers=host1:50051,host2:50052,host3:50053`
- Submit to one server, poll status from others:
  - Flags: `-submit-server=hostN:portN -status-servers=host1:port1,host2:port2`
  - Or env: `SUBMIT_SERVER=...` and `STATUS_SERVERS=host1:...,host2:...`

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





