package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"distributed-task-scheduler/internal/worker"

	"github.com/google/uuid"
)

func main() {
	// Generate unique worker ID
	workerId := uuid.New().String()

	// Create worker
	dsn := os.Getenv("DATABASE_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	w, err := worker.NewWorker(workerId, dsn, redisAddr)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}
	defer w.Close()

	// Create context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Start worker
	log.Printf("Starting worker %s...", workerId)
	if err := w.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Worker failed: %v", err)
	}
}
