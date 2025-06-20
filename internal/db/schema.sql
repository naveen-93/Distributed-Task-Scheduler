CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED')),
    command TEXT NOT NULL,
    output TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
); 