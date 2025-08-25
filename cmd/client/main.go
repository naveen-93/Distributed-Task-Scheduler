package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	pb "distributed-task-scheduler/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultServerAddr = "localhost:50051"
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
	serversFlag := flag.String("servers", "", "Comma-separated list of server addresses (overrides SERVERS env)")
	submitServerFlag := flag.String("submit-server", "", "Single server address to submit jobs to")
	statusServersFlag := flag.String("status-servers", "", "Comma-separated list of server addresses to poll status from")
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

	// Resolve server addresses (backward compatible)
	servers := []string{}
	if *serversFlag != "" {
		servers = splitAndTrim(*serversFlag)
	} else if env := os.Getenv("SERVERS"); env != "" {
		servers = splitAndTrim(env)
	}

	// Determine submit server
	submitServer := *submitServerFlag
	if submitServer == "" {
		if env := os.Getenv("SUBMIT_SERVER"); env != "" {
			submitServer = env
		} else if len(servers) > 0 {
			submitServer = servers[0]
		} else {
			submitServer = defaultServerAddr
		}
	}

	// Determine status servers
	statusServers := []string{}
	if *statusServersFlag != "" {
		statusServers = splitAndTrim(*statusServersFlag)
	} else if env := os.Getenv("STATUS_SERVERS"); env != "" {
		statusServers = splitAndTrim(env)
	} else if len(servers) > 0 {
		statusServers = servers
	} else {
		statusServers = []string{defaultServerAddr}
	}

	// Create submit client and status clients
	var conns []*grpc.ClientConn
	submitConn, err := grpc.Dial(submitServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to submit server %s: %v", submitServer, err)
	}
	conns = append(conns, submitConn)
	submitClient := pb.NewJobServiceClient(submitConn)

	var statusClients []pb.JobServiceClient
	for _, addr := range statusServers {
		c, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("Failed to connect to status server %s: %v", addr, err)
		}
		conns = append(conns, c)
		statusClients = append(statusClients, pb.NewJobServiceClient(c))
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	ctx := context.Background()

	if *concurrent {
		// Process jobs concurrently
		processJobsConcurrently(ctx, submitClient, statusClients, jobs.Jobs)
	} else {
		// Process jobs sequentially
		processJobsSequentially(ctx, submitClient, statusClients, jobs.Jobs)
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
func processJobsSequentially(ctx context.Context, submitClient pb.JobServiceClient, statusClients []pb.JobServiceClient, jobConfigs []JobConfig) {
	for i, jobConfig := range jobConfigs {
		log.Printf("\n=== Processing Job %d/%d: %s ===", i+1, len(jobConfigs), jobConfig.Name)
		if jobConfig.Description != "" {
			log.Printf("Description: %s", jobConfig.Description)
		}
		log.Printf("Command: %s", jobConfig.Command)

		result := processJob(ctx, submitClient, statusClients, i, jobConfig)
		printJobResult(result)
	}
}

// processJobsConcurrently processes all jobs simultaneously
func processJobsConcurrently(ctx context.Context, submitClient pb.JobServiceClient, statusClients []pb.JobServiceClient, jobConfigs []JobConfig) {
	var wg sync.WaitGroup
	results := make(chan JobResult, len(jobConfigs))

	// Submit all jobs concurrently
	for i, jobConfig := range jobConfigs {
		wg.Add(1)
		go func(idx int, config JobConfig) {
			defer wg.Done()
			result := processJob(ctx, submitClient, statusClients, idx, config)
			results <- result
		}(i, jobConfig)
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
func processJob(ctx context.Context, submitClient pb.JobServiceClient, statusClients []pb.JobServiceClient, idx int, jobConfig JobConfig) JobResult {
	result := JobResult{Config: jobConfig}

	// Submit job
	job := &pb.Job{
		Command:   jobConfig.Command,
		CreatedAt: time.Now().Unix(),
	}

	resp, err := submitClient.SubmitJob(ctx, job)
	if err != nil {
		result.Error = fmt.Errorf("failed to submit job: %v", err)
		return result
	}

	result.JobID = resp.JobId
	log.Printf("[%s] Job submitted successfully. Job ID: %s", jobConfig.Name, resp.JobId)

	// Poll for job status from status servers (round-robin)
	for attempt := 0; ; attempt++ {
		client := statusClients[(idx+attempt)%len(statusClients)]
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

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
