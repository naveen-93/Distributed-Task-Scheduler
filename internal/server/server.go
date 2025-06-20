package server

import (
	"context"
	"time"

	"distributed-task-scheduler/internal/db"
	"distributed-task-scheduler/internal/queue"
	pb "distributed-task-scheduler/proto"

	"github.com/google/uuid"
)

type JobServer struct {
	pb.UnimplementedJobServiceServer
	dbMgr    *db.DBManager
	queueMgr *queue.QueueManager
}

func NewJobServer(dbPath string, redisAddr string) (*JobServer, error) {
	dbMgr, err := db.NewDBManager(dbPath)
	if err != nil {
		return nil, err
	}

	queueMgr, err := queue.NewQueueManager(redisAddr)
	if err != nil {
		return nil, err
	}

	return &JobServer{
		dbMgr:    dbMgr,
		queueMgr: queueMgr,
	}, nil
}

func (s *JobServer) SubmitJob(ctx context.Context, job *pb.Job) (*pb.JobResponse, error) {
	// Generate unique ID if not provided
	if job.Id == "" {
		job.Id = uuid.New().String()
	}
	job.CreatedAt = time.Now().Unix()

	// Store job in database
	if err := s.dbMgr.CreateJob(job.Id, job.Command); err != nil {
		return nil, err
	}

	// Push to queue
	if err := s.queueMgr.PushJob(ctx, job.Id); err != nil {
		return nil, err
	}

	return &pb.JobResponse{
		JobId:   job.Id,
		Success: true,
		Message: "Job submitted successfully",
	}, nil
}

func (s *JobServer) GetJobStatus(ctx context.Context, jobId *pb.JobId) (*pb.JobStatus, error) {
	job, err := s.dbMgr.GetJob(jobId.Id)
	if err != nil {
		return nil, err
	}

	return &pb.JobStatus{
		Id:        job.ID,
		Status:    job.Status,
		Output:    job.Output.String,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}, nil
}

func (s *JobServer) Close() error {
	if err := s.dbMgr.Close(); err != nil {
		return err
	}
	return s.queueMgr.Close()
}
