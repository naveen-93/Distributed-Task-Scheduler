package queue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const PENDING_JOBS_QUEUE = "pending_jobs"

type QueueManager struct {
	client *redis.Client
}

func NewQueueManager(addr string) (*QueueManager, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &QueueManager{client: client}, nil
}

func (m *QueueManager) PushJob(ctx context.Context, jobId string) error {
	return m.client.LPush(ctx, PENDING_JOBS_QUEUE, jobId).Err()
}

func (m *QueueManager) PopJob(ctx context.Context) (string, error) {
	// BRPOP blocks until a job is available
	result, err := m.client.BRPop(ctx, 0, PENDING_JOBS_QUEUE).Result()
	if err != nil {
		return "", err
	}
	// Result contains [key, value], we want value (jobId)
	return result[1], nil
}

func (m *QueueManager) Close() error {
	return m.client.Close()
}
