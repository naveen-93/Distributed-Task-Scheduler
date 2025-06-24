package main

import (
	"context"
	"flag"
	"log"
	"time"

	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	serverAddr = "localhost:50051"
)

func main() {
	// Command line flags
	command := flag.String("cmd", "python3 -c \"import time; start=time.time(); fib=lambda n: n if n<=1 else fib(n-1)+fib(n-2); result=fib(10); print(f'fib(35)={result}, took {time.time()-start:.2f}s')\"", "Command to execute")
	flag.Parse()

	// Connect to server
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewJobServiceClient(conn)
	ctx := context.Background()

	// Submit job
	job := &pb.Job{
		Command:   *command,
		CreatedAt: time.Now().Unix(),
	}

	resp, err := client.SubmitJob(ctx, job)
	if err != nil {
		log.Fatalf("Failed to submit job: %v", err)
	}

	log.Printf("Job submitted successfully. Job ID: %s", resp.JobId)

	// Poll for job status
	for {
		status, err := client.GetJobStatus(ctx, &pb.JobId{Id: resp.JobId})
		if err != nil {
			log.Printf("Failed to get job status: %v", err)
			time.Sleep(time.Second)
			continue
		}

		log.Printf("Job status: %s", status.Status)
		if status.Status == "SUCCEEDED" || status.Status == "FAILED" {
			if status.Output != "" {
				log.Printf("Job output:\n%s", status.Output)
			}
			break
		}

		time.Sleep(time.Second)
	}
}
