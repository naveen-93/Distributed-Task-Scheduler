package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	serverAddr = "localhost:50051"
)

// JobConfig represents a single job configuration from JSON
type JobConfig struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

// JobsFile represents the structure of the JSON configuration file
type JobsFile struct {
	Jobs []JobConfig `json:"jobs"`
}

// JobResult holds the result of a submitted job
type JobResult struct {
	Config JobConfig
	JobID  string
	Status string
	Output string
	Error  error
}

func main() {
	// Command line flags
	jsonFile := flag.String("file", "jobs.json", "JSON file containing jobs to execute")
	concurrent := flag.Bool("concurrent", false, "Run jobs concurrently instead of sequentially")
	flag.Parse()

	// Read and parse JSON file
	jobs, err := loadJobsFromFile(*jsonFile)
	if err != nil {
		log.Fatalf("Failed to load jobs from file %s: %v", *jsonFile, err)
	}

	if len(jobs.Jobs) == 0 {
		log.Fatalf("No jobs found in file %s", *jsonFile)
	}

	log.Printf("Loaded %d jobs from %s", len(jobs.Jobs), *jsonFile)

	// Connect to server
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewJobServiceClient(conn)
	ctx := context.Background()

	if *concurrent {
		// Process jobs concurrently
		processJobsConcurrently(ctx, client, jobs.Jobs)
	} else {
		// Process jobs sequentially
		processJobsSequentially(ctx, client, jobs.Jobs)
	}
}

// loadJobsFromFile reads and parses the JSON configuration file
func loadJobsFromFile(filename string) (*JobsFile, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var jobs JobsFile
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &jobs, nil
}

// processJobsSequentially processes jobs one by one
func processJobsSequentially(ctx context.Context, client pb.JobServiceClient, jobConfigs []JobConfig) {
	for i, jobConfig := range jobConfigs {
		log.Printf("\n=== Processing Job %d/%d: %s ===", i+1, len(jobConfigs), jobConfig.Name)
		if jobConfig.Description != "" {
			log.Printf("Description: %s", jobConfig.Description)
		}
		log.Printf("Command: %s", jobConfig.Command)

		result := processJob(ctx, client, jobConfig)
		printJobResult(result)
	}
}

// processJobsConcurrently processes all jobs simultaneously
func processJobsConcurrently(ctx context.Context, client pb.JobServiceClient, jobConfigs []JobConfig) {
	var wg sync.WaitGroup
	results := make(chan JobResult, len(jobConfigs))

	// Submit all jobs concurrently
	for _, jobConfig := range jobConfigs {
		wg.Add(1)
		go func(config JobConfig) {
			defer wg.Done()
			result := processJob(ctx, client, config)
			results <- result
		}(jobConfig)
	}

	// Wait for all jobs to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and display results
	log.Printf("\n=== Concurrent Job Results ===")
	var allResults []JobResult
	for result := range results {
		allResults = append(allResults, result)
	}

	// Sort results by job name for consistent output
	for _, result := range allResults {
		printJobResult(result)
	}
}

// processJob submits a single job and waits for completion
func processJob(ctx context.Context, client pb.JobServiceClient, jobConfig JobConfig) JobResult {
	result := JobResult{Config: jobConfig}

	// Submit job
	job := &pb.Job{
		Command:   jobConfig.Command,
		CreatedAt: time.Now().Unix(),
	}

	resp, err := client.SubmitJob(ctx, job)
	if err != nil {
		result.Error = fmt.Errorf("failed to submit job: %v", err)
		return result
	}

	result.JobID = resp.JobId
	log.Printf("[%s] Job submitted successfully. Job ID: %s", jobConfig.Name, resp.JobId)

	// Poll for job status
	for {
		status, err := client.GetJobStatus(ctx, &pb.JobId{Id: resp.JobId})
		if err != nil {
			log.Printf("[%s] Failed to get job status: %v", jobConfig.Name, err)
			time.Sleep(time.Second)
			continue
		}

		result.Status = status.Status
		result.Output = status.Output

		if status.Status == "SUCCEEDED" || status.Status == "FAILED" {
			break
		}

		time.Sleep(time.Second)
	}

	return result
}

// printJobResult displays the result of a job execution
func printJobResult(result JobResult) {
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("Job: %s\n", result.Config.Name)
	fmt.Printf("Command: %s\n", result.Config.Command)
	fmt.Printf("Job ID: %s\n", result.JobID)

	if result.Error != nil {
		fmt.Printf("Status: ERROR\n")
		fmt.Printf("Error: %v\n", result.Error)
	} else {
		fmt.Printf("Status: %s\n", result.Status)
		if result.Output != "" {
			fmt.Printf("Output:\n%s\n", result.Output)
		}
	}
	fmt.Printf(strings.Repeat("=", 60) + "\n")
}
