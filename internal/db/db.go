package db

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Job struct {
	ID         string
	Status     string
	Command    string
	Output     sql.NullString
	CreatedAt  int64
	UpdatedAt  int64
	Retries    int32
	MaxRetries int32
	CronExpr   sql.NullString
	NextRunAt  sql.NullTime
}

type DBManager struct {
	pool *pgxpool.Pool
}

// NewDBManager initializes a PostgreSQL connection pool and ensures schema exists.
// dsn example: postgres://user:pass@localhost:5432/scheduler?sslmode=disable
func NewDBManager(dsn string) (*DBManager, error) {
	ctx := context.Background()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// Optional pooling controls via env
	if v := os.Getenv("PG_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			config.MaxConns = int32(n)
		}
	}
	if v := os.Getenv("PG_MIN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			config.MinConns = int32(n)
		}
	}
	if v := os.Getenv("PG_MAX_CONN_LIFETIME"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			config.MaxConnLifetime = d
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	mgr := &DBManager{pool: pool}
	if err := mgr.initDB(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return mgr, nil
}

func (m *DBManager) initDB(ctx context.Context) error {
	// Base tables
	_, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			name TEXT,
			args JSONB,
			command TEXT,
			execute_at TIMESTAMPTZ,
			status TEXT NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED')),
			retries INTEGER NOT NULL DEFAULT 0,
			priority INTEGER NOT NULL DEFAULT 0,
			output TEXT,
			created_at BIGINT NOT NULL,
			updated_at BIGINT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS task_history (
			id BIGSERIAL PRIMARY KEY,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			result TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		return err
	}
	// Add new columns if missing
	_, _ = m.pool.Exec(ctx, `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS max_retries INTEGER NOT NULL DEFAULT 3`)
	_, _ = m.pool.Exec(ctx, `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS cron_expr TEXT`)
	_, _ = m.pool.Exec(ctx, `ALTER TABLE tasks ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ`)
	return nil
}

func (m *DBManager) CreateJob(id, command string) error {
	ctx := context.Background()
	now := time.Now().Unix()
	_, err := m.pool.Exec(ctx,
		`INSERT INTO tasks (id, name, args, command, execute_at, status, retries, priority, output, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, 0, 0, NULL, $7, $8)`,
		id, "shell", nil, command, nil, "PENDING", now, now,
	)
	return err
}

func (m *DBManager) UpdateJobStatus(id, status string, output string) error {
	ctx := context.Background()
	now := time.Now().Unix()

	// Update task row
	_, err := m.pool.Exec(ctx,
		`UPDATE tasks SET status = $1, output = $2, updated_at = $3 WHERE id = $4`,
		status, nullableString(output), now, id,
	)
	if err != nil {
		return err
	}

	// Insert history row
	var start, end *time.Time
	nowTime := time.Now()
	if status == "RUNNING" {
		start = &nowTime
	} else if status == "SUCCEEDED" || status == "FAILED" {
		end = &nowTime
	}
	_, _ = m.pool.Exec(ctx,
		`INSERT INTO task_history (task_id, status, start_time, end_time, result) VALUES ($1, $2, $3, $4, $5)`,
		id, status, start, end, nullableString(output),
	)

	return nil
}

func (m *DBManager) GetJob(id string) (*Job, error) {
	ctx := context.Background()
	job := &Job{}
	var output sql.NullString
	var cron sql.NullString
	var next sql.NullTime
	err := m.pool.QueryRow(ctx,
		`SELECT id, status, COALESCE(command, ''), output, created_at, updated_at, retries, max_retries, cron_expr, next_run_at FROM tasks WHERE id = $1`,
		id,
	).Scan(&job.ID, &job.Status, &job.Command, &output, &job.CreatedAt, &job.UpdatedAt, &job.Retries, &job.MaxRetries, &cron, &next)
	if err != nil {
		return nil, err
	}
	job.Output = output
	job.CronExpr = cron
	job.NextRunAt = next
	return job, nil
}

func (m *DBManager) Close() error {
	if m.pool != nil {
		m.pool.Close()
	}
	return nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// IncrementRetry increments retries and returns (retries, max_retries)
func (m *DBManager) IncrementRetry(id string) (int32, int32, error) {
	ctx := context.Background()
	var retries int32
	var max int32
	err := m.pool.QueryRow(ctx, `UPDATE tasks SET retries = retries + 1, updated_at = $2 WHERE id=$1 RETURNING retries, max_retries`, id, time.Now().Unix()).Scan(&retries, &max)
	if err != nil {
		return 0, 0, err
	}
	return retries, max, nil
}

// ResetToPending sets status back to PENDING and updates output/updated_at
func (m *DBManager) ResetToPending(id string, output string) error {
	ctx := context.Background()
	_, err := m.pool.Exec(ctx, `UPDATE tasks SET status='PENDING', output=$2, updated_at=$3 WHERE id=$1`, id, nullableString(output), time.Now().Unix())
	return err
}

// GetDueTaskIDs returns task IDs due for enqueue (one-time execute_at or cron next_run_at)
func (m *DBManager) GetDueTaskIDs(limit int) ([]string, error) {
	ctx := context.Background()
	rows, err := m.pool.Query(ctx, `
		SELECT id FROM tasks
		WHERE status='PENDING' AND (
		  execute_at IS NULL OR execute_at <= now() OR (next_run_at IS NOT NULL AND next_run_at <= now())
		)
		ORDER BY updated_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateNextRun sets next_run_at for cron tasks
func (m *DBManager) UpdateNextRun(id string, t time.Time) error {
	ctx := context.Background()
	_, err := m.pool.Exec(ctx, `UPDATE tasks SET next_run_at=$2, updated_at=$3 WHERE id=$1`, id, t, time.Now().Unix())
	return err
}

// ClearExecuteAt nulls execute_at to prevent re-enqueue of one-time tasks after push
func (m *DBManager) ClearExecuteAt(id string) error {
	ctx := context.Background()
	_, err := m.pool.Exec(ctx, `UPDATE tasks SET execute_at=NULL, updated_at=$2 WHERE id=$1`, id, time.Now().Unix())
	return err
}

// MarkStaleRunningJobsFailed marks RUNNING tasks as FAILED if updated_at older than cutoffSeconds.
func (m *DBManager) MarkStaleRunningJobsFailed(cutoffSeconds int64) (int64, error) {
	ctx := context.Background()
	now := time.Now().Unix()
	cutoff := now - cutoffSeconds
	cmdTag, err := m.pool.Exec(ctx,
		`UPDATE tasks SET status='FAILED', output=COALESCE(output,'') || '\n[auto] marked failed due to staleness', updated_at=$1
		 WHERE status='RUNNING' AND updated_at < $2`, now, cutoff,
	)
	if err != nil {
		return 0, err
	}
	return cmdTag.RowsAffected(), nil
}
