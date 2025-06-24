package queue

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	PENDING_JOBS_QUEUE = "pending_jobs"
	RECONNECT_DELAY    = 5 * time.Second
	POP_TIMEOUT        = 5 * time.Second
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

	// For debugging, let's see what's currently in the queue
	length, err := m.client.LLen(ctx, PENDING_JOBS_QUEUE).Result()
	if err != nil {
		log.Printf("Error getting queue length: %v", err)
		return err
	}
	log.Printf("Current queue length before push: %d", length)

	// Try the push operation - let's use background context to rule out context issues
	backgroundCtx := context.Background()
	log.Printf("Executing RPUSH for job %s with background context", jobId)
	pushResult := m.client.RPush(backgroundCtx, PENDING_JOBS_QUEUE, jobId)
	if err := pushResult.Err(); err != nil {
		log.Printf("RPUSH failed for job %s: %v", jobId, err)
		return err
	}

	// Get the result of RPUSH (should be new list length)
	newLength, err := pushResult.Result()
	if err != nil {
		log.Printf("Error getting RPUSH result: %v", err)
		return err
	}
	log.Printf("RPUSH successful for job %s, new length: %d", jobId, newLength)

	// Add small delay to see if worker pops the job immediately
	log.Printf("Waiting 100ms before verification...")
	time.Sleep(100 * time.Millisecond)

	// Double-check by querying the queue again with background context
	actualLength, err := m.client.LLen(backgroundCtx, PENDING_JOBS_QUEUE).Result()
	if err != nil {
		log.Printf("Error getting actual queue length: %v", err)
	} else {
		log.Printf("Verification: actual queue length after push: %d", actualLength)
	}

	// For debugging, let's see what's in the queue now
	queueContents, err := m.client.LRange(backgroundCtx, PENDING_JOBS_QUEUE, 0, -1).Result()
	if err != nil {
		log.Printf("Error getting queue contents: %v", err)
	} else {
		log.Printf("Queue contents after push: %v", queueContents)
	}

	// Let's also try a direct Redis command to double-check
	log.Printf("Testing direct Redis command...")
	testResult := m.client.RPush(backgroundCtx, "test_from_go", jobId)
	if err := testResult.Err(); err != nil {
		log.Printf("Direct Redis test failed: %v", err)
	} else {
		testLength, _ := testResult.Result()
		log.Printf("Direct Redis test successful, length: %d", testLength)
	}

	return nil
}

func (m *QueueManager) PopJob(ctx context.Context) (string, error) {
	if err := m.ensureConnected(ctx); err != nil {
		return "", err
	}

	// Use background context for Redis operations to avoid context cancellation issues
	backgroundCtx := context.Background()

	// For debugging, let's see what's currently in the queue
	currentLength, _ := m.client.LLen(backgroundCtx, PENDING_JOBS_QUEUE).Result()
	log.Printf("PopJob: Current queue length before pop: %d", currentLength)

	if currentLength > 0 {
		queueContents, _ := m.client.LRange(backgroundCtx, PENDING_JOBS_QUEUE, 0, -1).Result()
		log.Printf("PopJob: Queue contents before pop: %v", queueContents)
	}

	// Use BLPOP to remove from the front of the list (FIFO order)
	// This will block until a job is available or timeout occurs
	log.Printf("PopJob: Starting BLPOP with timeout %v using background context", POP_TIMEOUT)
	result, err := m.client.BLPop(backgroundCtx, POP_TIMEOUT, PENDING_JOBS_QUEUE).Result()
	if err != nil {
		if err == redis.Nil {
			log.Printf("No jobs available in queue after %v timeout", POP_TIMEOUT)
			return "", ErrQueueTimeout
		}

		log.Printf("PopJob: BLPOP error: %v", err)
		// Try to reconnect on error
		if err := m.connect(); err != nil {
			log.Printf("Failed to reconnect to Redis: %v", err)
			return "", err
		}

		return "", err
	}

	// Result contains [key, value], we want value (jobId)
	jobId := result[1]
	log.Printf("PopJob: BLPOP returned result: %v, extracted jobId: %s", result, jobId)

	// Log remaining queue length
	length, _ := m.client.LLen(backgroundCtx, PENDING_JOBS_QUEUE).Result()
	log.Printf("Successfully popped job %s from queue. Remaining queue length: %d", jobId, length)

	return jobId, nil
}

func (m *QueueManager) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
