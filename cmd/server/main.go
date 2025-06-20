package main

import (
	"log"
	"net"

	"distributed-task-scheduler/internal/server"
	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
)

const (
	port      = ":50051"
	dbPath    = "jobs.db"
	redisAddr = "localhost:6379"
)

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create job server
	jobServer, err := server.NewJobServer(dbPath, redisAddr)
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
