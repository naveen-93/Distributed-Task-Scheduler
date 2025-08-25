package worker

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"distributed-task-scheduler/internal/db"
	"distributed-task-scheduler/internal/queue"
)

const (
	RECONNECT_DELAY = 5 * time.Second
	MAX_RETRIES     = 3
)

type Worker struct {
	id        string
	dbMgr     *db.DBManager
	queueMgr  *queue.QueueManager
	redisAddr string
}

func NewWorker(id string, dsn string, redisAddr string) (*Worker, error) {
	log.Printf("Initializing worker %s with DB: %s, Redis: %s", id, dsn, redisAddr)

	dbMgr, err := db.NewDBManager(dsn)
	if err != nil {
		log.Printf("Failed to initialize database: %v", err)
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	queueMgr, err := queue.NewQueueManager(redisAddr)
	if err != nil {
		log.Printf("Failed to initialize Redis queue: %v", err)
		dbMgr.Close() // Clean up database connection
		return nil, fmt.Errorf("failed to initialize Redis queue: %v", err)
	}

	log.Printf("Worker %s initialized successfully", id)
	return &Worker{
		id:        id,
		dbMgr:     dbMgr,
		queueMgr:  queueMgr,
		redisAddr: redisAddr,
	}, nil
}

func (w *Worker) Start(ctx context.Context) error {
	log.Printf("Worker %s starting... Waiting for jobs", w.id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %s shutting down...", w.id)
			return ctx.Err()
		default:
			if err := w.processNextJob(ctx); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return err
				}

				// Handle queue errors
				if err == queue.ErrQueueTimeout {

					continue
				}

				log.Printf("Error processing job: %v", err)
				// Add delay before retrying on error
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(RECONNECT_DELAY):
					continue
				}
			}
		}
	}
}

func (w *Worker) processNextJob(ctx context.Context) error {
	// Get next job from queue with retries
	var jobId string
	var err error

	for retries := 0; retries < MAX_RETRIES; retries++ {
		log.Printf("Worker %s waiting for next job (attempt %d/%d)...", w.id, retries+1, MAX_RETRIES)
		jobId, err = w.queueMgr.PopJob(ctx)
		if err == nil {
			break
		}
		if err == queue.ErrQueueTimeout {
			return err // Normal timeout, caller will continue
		}
		log.Printf("Worker %s: error getting job from queue (attempt %d/%d): %v", w.id, retries+1, MAX_RETRIES, err)
		if retries < MAX_RETRIES-1 {
			time.Sleep(RECONNECT_DELAY)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to get job after %d attempts: %v", MAX_RETRIES, err)
	}

	log.Printf("Worker %s received job %s", w.id, jobId)

	// Get job details from database with retries
	var job *db.Job
	for retries := 0; retries < MAX_RETRIES; retries++ {
		job, err = w.dbMgr.GetJob(jobId)
		if err == nil {
			break
		}
		log.Printf("Worker %s failed to get job details for %s (attempt %d/%d): %v", w.id, jobId, retries+1, MAX_RETRIES, err)
		if retries < MAX_RETRIES-1 {
			time.Sleep(RECONNECT_DELAY)
		}
	}
	if err != nil {
		log.Printf("Worker %s: giving up on job %s after %d attempts", w.id, jobId, MAX_RETRIES)
		// Mark job as failed if we can't get details
		w.dbMgr.UpdateJobStatus(jobId, "FAILED", fmt.Sprintf("Failed to retrieve job details: %v", err))
		return fmt.Errorf("failed to get job details after %d attempts: %v", MAX_RETRIES, err)
	}

	log.Printf("Worker %s processing job %s: %s", w.id, jobId, job.Command)

	// Update status to RUNNING
	if err := w.dbMgr.UpdateJobStatus(jobId, "RUNNING", ""); err != nil {
		log.Printf("Worker %s failed to update job %s to RUNNING: %v", w.id, jobId, err)
		return fmt.Errorf("failed to update job status: %v", err)
	}

	// Execute the command with context
	cmd := exec.CommandContext(ctx, "sh", "-c", job.Command)
	output, err := cmd.CombinedOutput()

	// Update job status based on execution result
	status := "SUCCEEDED"
	outputStr := string(output)

	if err != nil {
		status = "FAILED"
		if ctx.Err() != nil {
			// Job was cancelled due to context
			outputStr = "Job cancelled: " + outputStr
			log.Printf("Worker %s: job %s cancelled", w.id, jobId)
		} else {
			log.Printf("Worker %s: job %s failed: %v", w.id, jobId, err)
			outputStr = fmt.Sprintf("Error: %v\nOutput: %s", err, outputStr)
		}
	} else {
		log.Printf("Worker %s: job %s completed successfully", w.id, jobId)
	}

	// Update final status and output with retries
	for retries := 0; retries < MAX_RETRIES; retries++ {
		if err := w.dbMgr.UpdateJobStatus(jobId, status, outputStr); err == nil {
			break
		}
		log.Printf("Worker %s failed to update final status for job %s (attempt %d/%d): %v", w.id, jobId, retries+1, MAX_RETRIES, err)
		if retries < MAX_RETRIES-1 {
			time.Sleep(RECONNECT_DELAY)
		}
	}

	return nil
}

func (w *Worker) Close() error {
	log.Printf("Worker %s cleaning up...", w.id)
	var dbErr, queueErr error

	if w.dbMgr != nil {
		dbErr = w.dbMgr.Close()
		if dbErr != nil {
			log.Printf("Error closing database connection: %v", dbErr)
		}
	}
	if w.queueMgr != nil {
		queueErr = w.queueMgr.Close()
		if queueErr != nil {
			log.Printf("Error closing queue connection: %v", queueErr)
		}
	}

	// Return the first error encountered
	if dbErr != nil {
		return dbErr
	}
	return queueErr
}
