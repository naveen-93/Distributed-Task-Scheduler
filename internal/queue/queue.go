package queue

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	PENDING_JOBS_QUEUE    = "pending_jobs"
	PROCESSING_JOBS_QUEUE = "processing_jobs"
	DLQ_JOBS_QUEUE        = "dlq_tasks"
	RECONNECT_DELAY       = 5 * time.Second
	POP_TIMEOUT           = 5 * time.Second
)

var (
	ErrQueueTimeout      = errors.New("queue timeout")
	ErrRedisNotConnected = errors.New("redis not connected")
	ErrJobAlreadyQueued  = errors.New("job already in queue")
)

type QueueManager struct {
	client *redis.Client
	addr   string
}

func NewQueueManager(addr string) (*QueueManager, error) {
	qm := &QueueManager{
		addr: addr,
	}

	if err := qm.connect(); err != nil {
		return nil, err
	}

	return qm, nil
}

func (m *QueueManager) connect() error {
	if m.client != nil {
		m.client.Close()
	}

	m.client = redis.NewClient(&redis.Options{
		Addr: m.addr,
		DB:   0,
	})

	// Test connection
	ctx := context.Background()
	if err := m.client.Ping(ctx).Err(); err != nil {
		m.client.Close()
		m.client = nil
		return err
	}

	log.Printf("Successfully connected to Redis at %s", m.addr)
	return nil
}

func (m *QueueManager) ensureConnected(ctx context.Context) error {
	if m.client == nil {
		return m.connect()
	}

	// Test connection
	if err := m.client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection lost, attempting to reconnect: %v", err)
		return m.connect()
	}

	return nil
}

func (m *QueueManager) PushJob(ctx context.Context, jobId string) error {
	if err := m.ensureConnected(ctx); err != nil {
		log.Printf("Connection error in PushJob: %v", err)
		return err
	}

	log.Printf("Attempting to push job %s to queue", jobId)

	// Test Redis connection with a simple ping
	if err := m.client.Ping(ctx).Err(); err != nil {
		log.Printf("Redis ping failed in PushJob: %v", err)
		return err
	}
	log.Printf("Redis ping successful before push")

	// Try the push operation - background context to avoid cancellation issues
	backgroundCtx := context.Background()
	pushResult := m.client.RPush(backgroundCtx, PENDING_JOBS_QUEUE, jobId)
	if err := pushResult.Err(); err != nil {
		log.Printf("RPUSH failed for job %s: %v", jobId, err)
		return err
	}
	_, _ = pushResult.Result()
	return nil
}

func (m *QueueManager) PopJob(ctx context.Context) (string, error) {
	if err := m.ensureConnected(ctx); err != nil {
		return "", err
	}

	// Use background context for Redis operations to avoid context cancellation issues
	backgroundCtx := context.Background()

	// Atomically move from pending -> processing
	log.Printf("PopJob: BRPOPLPUSH from %s to %s with timeout %v", PENDING_JOBS_QUEUE, PROCESSING_JOBS_QUEUE, POP_TIMEOUT)
	jobId, err := m.client.BRPopLPush(backgroundCtx, PENDING_JOBS_QUEUE, PROCESSING_JOBS_QUEUE, POP_TIMEOUT).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("No jobs available in queue after %v timeout", POP_TIMEOUT)
			return "", ErrQueueTimeout
		}

		log.Printf("PopJob: BRPOPLPUSH error: %v", err)
		// Try to reconnect on error
		if err := m.connect(); err != nil {
			log.Printf("Failed to reconnect to Redis: %v", err)
			return "", err
		}

		return "", err
	}

	log.Printf("PopJob: moved job %s to processing queue", jobId)
	return jobId, nil
}

func (m *QueueManager) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// AckProcessing removes a processed jobId from the processing queue.
func (m *QueueManager) AckProcessing(ctx context.Context, jobId string) error {
	if err := m.ensureConnected(ctx); err != nil {
		return err
	}
	_, err := m.client.LRem(ctx, PROCESSING_JOBS_QUEUE, 1, jobId).Result()
	return err
}

// RequeueFromProcessing moves a job back to pending and removes it from processing.
func (m *QueueManager) RequeueFromProcessing(ctx context.Context, jobId string) error {
	if err := m.ensureConnected(ctx); err != nil {
		return err
	}
	if _, err := m.client.LRem(ctx, PROCESSING_JOBS_QUEUE, 1, jobId).Result(); err != nil {
		return err
	}
	return m.client.RPush(ctx, PENDING_JOBS_QUEUE, jobId).Err()
}

// MoveToDLQ moves a job to DLQ and removes it from processing.
func (m *QueueManager) MoveToDLQ(ctx context.Context, jobId string) error {
	if err := m.ensureConnected(ctx); err != nil {
		return err
	}
	if _, err := m.client.LRem(ctx, PROCESSING_JOBS_QUEUE, 1, jobId).Result(); err != nil {
		return err
	}
	return m.client.RPush(ctx, DLQ_JOBS_QUEUE, jobId).Err()
}
