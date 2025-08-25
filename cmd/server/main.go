package main

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"distributed-task-scheduler/internal/coord"
	"distributed-task-scheduler/internal/server"
	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
)

func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "50051"
	}
	lis, err := net.Listen("tcp", ":"+port)
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

	// Leader election (optional)
	if eps := os.Getenv("ETCD_ENDPOINTS"); eps != "" {
		endpoints := strings.Split(eps, ",")
		ns := os.Getenv("ELECTION_NAMESPACE")
		if ns == "" {
			ns = "/scheduler/v1"
		}
		name := os.Getenv("ELECTION_KEY")
		if name == "" {
			name = "leader"
		}
		leaseTTL := 10 * time.Second
		if v := os.Getenv("LEASE_TTL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				leaseTTL = d
			}
		}
		leader := coord.NewEtcdLeader(endpoints, ns, name, leaseTTL)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			_ = leader.Run(ctx, coord.LeaderCallbacks{
				OnStartedLeading: func(lctx context.Context) {
					jobServer.StartLeaderLoops(lctx)
				},
			})
		}()
		log.Printf("Leader election enabled with endpoints=%v namespace=%s key=%s ttl=%s", endpoints, ns, name, leaseTTL)
	} else {
		log.Printf("Leader election disabled (ETCD_ENDPOINTS not set). Running without leader-only duties.")
	}

	// Create gRPC server
	s := grpc.NewServer()
	pb.RegisterJobServiceServer(s, jobServer)

	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
