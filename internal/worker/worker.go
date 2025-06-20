package worker

import (
	"context"
	"log"
	"os/exec"

	"distributed-task-scheduler/internal/db"
	"distributed-task-scheduler/internal/queue"
)

type Worker struct {
	id       string
	dbMgr    *db.DBManager
	queueMgr *queue.QueueManager
}

func NewWorker(id string, dbPath string, redisAddr string) (*Worker, error) {
	dbMgr, err := db.NewDBManager(dbPath)
	if err != nil {
		return nil, err
	}

	queueMgr, err := queue.NewQueueManager(redisAddr)
	if err != nil {
		return nil, err
	}

	return &Worker{
		id:       id,
		dbMgr:    dbMgr,
		queueMgr: queueMgr,
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
				if err == context.Canceled {
					return err
				}
				log.Printf("Error processing job: %v", err)
				// Continue processing next job even if current one fails
				continue
			}
		}
	}
}

func (w *Worker) processNextJob(ctx context.Context) error {
	// Get next job from queue
	log.Printf("Worker %s waiting for next job...", w.id)
	jobId, err := w.queueMgr.PopJob(ctx)
	if err != nil {
		return err
	}
	log.Printf("Worker %s received job %s", w.id, jobId)

	// Get job details from database
	job, err := w.dbMgr.GetJob(jobId)
	if err != nil {
		log.Printf("Worker %s failed to get job details for %s: %v", w.id, jobId, err)
		return err
	}
	log.Printf("Worker %s processing job %s: %s", w.id, jobId, job.Command)

	// Update status to RUNNING
	if err := w.dbMgr.UpdateJobStatus(jobId, "RUNNING", ""); err != nil {
		log.Printf("Worker %s failed to update job %s to RUNNING: %v", w.id, jobId, err)
		return err
	}

	// Execute the command
	cmd := exec.Command("sh", "-c", job.Command)
	output, err := cmd.CombinedOutput()

	// Update job status based on execution result
	status := "SUCCEEDED"
	if err != nil {
		status = "FAILED"
		log.Printf("Worker %s: job %s failed: %v", w.id, jobId, err)
	} else {
		log.Printf("Worker %s: job %s completed successfully", w.id, jobId)
	}

	// Update final status and output
	if err := w.dbMgr.UpdateJobStatus(jobId, status, string(output)); err != nil {
		log.Printf("Worker %s failed to update final status for job %s: %v", w.id, jobId, err)
		return err
	}

	return nil
}

func (w *Worker) Close() error {
	log.Printf("Worker %s cleaning up...", w.id)
	if err := w.dbMgr.Close(); err != nil {
		return err
	}
	return w.queueMgr.Close()
}
