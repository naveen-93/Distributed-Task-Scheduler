package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Job struct {
	ID        string
	Status    string
	Command   string
	Output    sql.NullString
	CreatedAt int64
	UpdatedAt int64
}

type DBManager struct {
	db *sql.DB
}

func NewDBManager(dbPath string) (*DBManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	if err := initDB(db); err != nil {
		db.Close()
		return nil, err
	}

	return &DBManager{db: db}, nil
}

func initDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		status TEXT NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED')),
		command TEXT NOT NULL,
		output TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`

	_, err := db.Exec(schema)
	return err
}

func (m *DBManager) CreateJob(id, command string) error {
	now := time.Now().Unix()
	_, err := m.db.Exec(
		"INSERT INTO jobs (id, status, command, output, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, "PENDING", command, nil, now, now,
	)
	return err
}

func (m *DBManager) UpdateJobStatus(id, status string, output string) error {
	now := time.Now().Unix()
	var outputVal sql.NullString
	if output != "" {
		outputVal = sql.NullString{String: output, Valid: true}
	}
	_, err := m.db.Exec(
		"UPDATE jobs SET status = ?, output = ?, updated_at = ? WHERE id = ?",
		status, outputVal, now, id,
	)
	return err
}

func (m *DBManager) GetJob(id string) (*Job, error) {
	job := &Job{}
	err := m.db.QueryRow(
		"SELECT id, status, command, output, created_at, updated_at FROM jobs WHERE id = ?",
		id,
	).Scan(&job.ID, &job.Status, &job.Command, &job.Output, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (m *DBManager) Close() error {
	return m.db.Close()
}