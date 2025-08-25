package server

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"distributed-task-scheduler/internal/db"
	"distributed-task-scheduler/internal/queue"
	pb "distributed-task-scheduler/proto"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type JobServer struct {
	pb.UnimplementedJobServiceServer
	dbMgr      *db.DBManager
	queueMgr   *queue.QueueManager
	stopLeader chan struct{}
}

func NewJobServer(dsn string, redisAddr string) (*JobServer, error) {
	log.Printf("Initializing job server with DB: %s, Redis: %s", dsn, redisAddr)

	dbMgr, err := db.NewDBManager(dsn)
	if err != nil {
		log.Printf("Failed to initialize database: %v", err)
		return nil, err
	}

	queueMgr, err := queue.NewQueueManager(redisAddr)
	if err != nil {
		log.Printf("Failed to initialize Redis queue: %v", err)
		dbMgr.Close() // Clean up database connection
		return nil, err
	}

	log.Printf("Job server initialized successfully")
	return &JobServer{
		dbMgr:      dbMgr,
		queueMgr:   queueMgr,
		stopLeader: make(chan struct{}),
	}, nil
}

// StartLeaderLoops runs leader-only maintenance until stop is closed or ctx done.
func (s *JobServer) StartLeaderLoops(ctx context.Context) {
	log.Printf("Leader duties started")
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	for {
		select {
		case <-ctx.Done():
			log.Printf("Leader duties stopping: context done")
			return
		case <-s.stopLeader:
			log.Printf("Leader duties stopping: stop signal")
			return
		case <-ticker.C:
			// 1) Mark stale RUNNING jobs as FAILED after 10 minutes of inactivity
			if n, err := s.dbMgr.MarkStaleRunningJobsFailed(600); err != nil {
				log.Printf("Leader maintenance error: %v", err)
			} else if n > 0 {
				log.Printf("Leader maintenance: marked %d stale RUNNING jobs as FAILED", n)
			}
			// 2) Enqueue due one-time and recurring tasks
			ids, err := s.dbMgr.GetDueTaskIDs(100)
			if err != nil {
				log.Printf("Leader enqueue scan error: %v", err)
				continue
			}
			for _, id := range ids {
				if err := s.queueMgr.PushJob(ctx, id); err != nil {
					log.Printf("Leader enqueue push error for %s: %v", id, err)
					continue
				}
				_ = s.dbMgr.ClearExecuteAt(id)
				// cron: compute next
				job, jerr := s.dbMgr.GetJob(id)
				if jerr == nil && job.CronExpr.Valid {
					if sched, perr := parser.Parse(job.CronExpr.String); perr == nil {
						next := sched.Next(time.Now())
						_ = s.dbMgr.UpdateNextRun(id, next)
					}
				}
			}
		}
	}
}

func (s *JobServer) SubmitJob(ctx context.Context, job *pb.Job) (*pb.JobResponse, error) {
	// Validate job command
	if strings.TrimSpace(job.Command) == "" {
		log.Printf("Received empty job command")
		return &pb.JobResponse{
			Success: false,
			Message: "Job command cannot be empty",
		}, errors.New("job command cannot be empty")
	}

	// Generate unique ID if not provided
	if job.Id == "" {
		job.Id = uuid.New().String()
	}
	job.CreatedAt = time.Now().Unix()

	log.Printf("Processing job submission - ID: %s, Command: %s", job.Id, job.Command)

	// Store job in database
	if err := s.dbMgr.CreateJob(job.Id, job.Command); err != nil {
		log.Printf("Failed to create job %s in database: %v", job.Id, err)
		return &pb.JobResponse{
			Success: false,
			Message: "Failed to create job in database",
		}, err
	}
	log.Printf("Job %s stored in database successfully", job.Id)

	// Push to queue - if this fails, we have a problem since job is already in DB
	if err := s.queueMgr.PushJob(ctx, job.Id); err != nil {
		log.Printf("Failed to push job %s to Redis queue: %v", job.Id, err)
		// Try to mark the job as failed since it's in DB but not in queue
		if updateErr := s.dbMgr.UpdateJobStatus(job.Id, "FAILED", "Failed to add job to processing queue"); updateErr != nil {
			log.Printf("Also failed to update job status in DB: %v", updateErr)
		}
		return &pb.JobResponse{
			JobId:   job.Id,
			Success: false,
			Message: "Failed to queue job for processing",
		}, err
	}
	log.Printf("Job %s queued for processing", job.Id)

	return &pb.JobResponse{
		JobId:   job.Id,
		Success: true,
		Message: "Job submitted successfully",
	}, nil
}

func (s *JobServer) GetJobStatus(ctx context.Context, jobId *pb.JobId) (*pb.JobStatus, error) {
	// Validate job ID
	if strings.TrimSpace(jobId.Id) == "" {
		log.Printf("Received empty job ID in status request")
		return nil, errors.New("job ID cannot be empty")
	}

	job, err := s.dbMgr.GetJob(jobId.Id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Job %s not found in database", jobId.Id)
			return nil, errors.New("job not found")
		}
		log.Printf("Error retrieving job %s from database: %v", jobId.Id, err)
		return nil, err
	}

	// Handle nullable output properly
	output := ""
	if job.Output.Valid {
		output = job.Output.String
	}

	log.Printf("Retrieved status for job %s: %s", jobId.Id, job.Status)
	return &pb.JobStatus{
		Id:        job.ID,
		Status:    job.Status,
		Output:    output,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}, nil
}

func (s *JobServer) Close() error {
	log.Print("Shutting down job server...")
	var dbErr, queueErr error

	if s.dbMgr != nil {
		dbErr = s.dbMgr.Close()
		if dbErr != nil {
			log.Printf("Error closing database connection: %v", dbErr)
		}
	}
	if s.queueMgr != nil {
		queueErr = s.queueMgr.Close()
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
