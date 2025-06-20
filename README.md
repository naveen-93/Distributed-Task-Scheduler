# Distributed Task Scheduler

A basic distributed task scheduler implementation in Go using gRPC and TCP sockets.

## Features

- Submit tasks with scheduled execution time
- Get task status and results
- Simple client-server architecture using gRPC
- Task execution scheduling

## Prerequisites

- Go 1.16 or later
- Protocol Buffers compiler (protoc)
- gRPC tools for Go

## Installation

1. Clone the repository:
```bash
git clone https://github.com/naveen-93/Distributed-Task-Scheduler.git
cd distributed-task-scheduler
```

2. Install dependencies:
```bash
go mod tidy
```

3. Generate protobuf code:
```bash
zsh protogen.sh
```

## Usage

1. Start the server:
```bash
go run main.go
```

2. Run the example client:
```bash
go run examples/main.go
```

The example client will:
- Submit a task to be executed in 5 seconds
- Poll for the task status until completion
- Display the task result

## Project Structure

- `proto/`: Protocol Buffer definitions
- `server/`: Server implementation
- `client/`: Client implementation
- `examples/`: Example usage of the task scheduler

## Contributing

Feel free to submit issues and enhancement requests! 