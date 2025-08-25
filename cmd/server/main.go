package main

import (
	"log"
	"net"
	"os"

	"distributed-task-scheduler/internal/server"
	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create job server
	dsn := os.Getenv("DATABASE_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	jobServer, err := server.NewJobServer(dsn, redisAddr)
	if err != nil {
		log.Fatalf("failed to create job server: %v", err)
	}
	defer jobServer.Close()

	// Create gRPC server
	s := grpc.NewServer()
	pb.RegisterJobServiceServer(s, jobServer)

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
