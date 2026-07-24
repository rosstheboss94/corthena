CREATE TABLE IF NOT EXISTS catalog_revisions (
    dataset_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    document TEXT NOT NULL,
    current INTEGER NOT NULL CHECK(current IN (0, 1)),
    PRIMARY KEY(dataset_id, revision)
);
CREATE UNIQUE INDEX IF NOT EXISTS one_current_revision
ON catalog_revisions(dataset_id) WHERE current = 1;
CREATE TABLE IF NOT EXISTS imports (
    operation_id TEXT PRIMARY KEY,
    command_id TEXT NOT NULL UNIQUE,
    correlation_id TEXT NOT NULL,
    generation INTEGER NOT NULL,
    state TEXT NOT NULL,
    document TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS schedules (
    schedule_id TEXT PRIMARY KEY,
    revision INTEGER NOT NULL,
    document TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS accepted_commands (
    command_id TEXT PRIMARY KEY,
    resource_kind TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    request_fingerprint TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS credential_status (
    provider TEXT PRIMARY KEY,
    last_tested_at TEXT,
    last_test_succeeded INTEGER,
    safe_detail TEXT
);
