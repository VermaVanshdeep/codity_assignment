-- =============================================================================
-- Migration 000001: Initial Schema
-- Creates the complete database schema for the Distributed Job Scheduler.
-- 
-- Conventions:
--   • All PKs are UUID v4 (app-generated, not DB-generated) for portability.
--   • All timestamps are TIMESTAMPTZ (UTC-aware) — never TIMESTAMP.
--   • ENUM types are defined as custom PostgreSQL types for DB-level safety.
--   • Partial indexes on status columns dramatically reduce index size.
-- =============================================================================

-- ─── Extensions ───────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";   -- for ILIKE searches on names

-- ─── ENUM Types ───────────────────────────────────────────────────────────────
CREATE TYPE org_role          AS ENUM ('owner', 'admin', 'member');
CREATE TYPE project_role      AS ENUM ('admin', 'developer', 'viewer');
CREATE TYPE job_status        AS ENUM (
    'pending', 'scheduled', 'running',
    'succeeded', 'failed', 'dead', 'cancelled'
);
CREATE TYPE retry_strategy    AS ENUM ('fixed', 'linear', 'exponential');
CREATE TYPE worker_status     AS ENUM ('active', 'draining', 'offline');
CREATE TYPE execution_status  AS ENUM ('started', 'succeeded', 'failed', 'timed_out');
CREATE TYPE log_level         AS ENUM ('debug', 'info', 'warn', 'error');
CREATE TYPE batch_status      AS ENUM ('pending', 'running', 'completed', 'partial_failure', 'cancelled');

-- =============================================================================
-- AUTHENTICATION
-- =============================================================================

CREATE TABLE users (
    id              UUID        PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    full_name       VARCHAR(255) NOT NULL DEFAULT '',
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users (email);

CREATE TABLE refresh_tokens (
    id          UUID        PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked     BOOLEAN     NOT NULL DEFAULT FALSE,
    user_agent  VARCHAR(512),
    ip_address  VARCHAR(45),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id    ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX idx_refresh_tokens_active     ON refresh_tokens (user_id, expires_at)
    WHERE revoked = FALSE;

-- =============================================================================
-- ORGANIZATIONS
-- =============================================================================

CREATE TABLE organizations (
    id          UUID        PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organizations_slug ON organizations (slug);

CREATE TABLE org_members (
    org_id      UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    role        org_role    NOT NULL DEFAULT 'member',
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, user_id)
);

CREATE INDEX idx_org_members_user_id ON org_members (user_id);

-- =============================================================================
-- PROJECTS
-- =============================================================================

CREATE TABLE projects (
    id              UUID        PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL,
    description     TEXT,
    api_key_hash    VARCHAR(255) UNIQUE,    -- hashed API key for programmatic access
    api_key_prefix  VARCHAR(20),            -- first 8 chars shown in UI (e.g. "sk_live_")
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, slug)
);

CREATE INDEX idx_projects_org_id      ON projects (org_id);
CREATE INDEX idx_projects_api_key     ON projects (api_key_hash) WHERE api_key_hash IS NOT NULL;

CREATE TABLE project_members (
    project_id  UUID            NOT NULL REFERENCES projects(id)  ON DELETE CASCADE,
    user_id     UUID            NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    role        project_role    NOT NULL DEFAULT 'developer',
    joined_at   TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user_id ON project_members (user_id);

-- =============================================================================
-- QUEUES
-- =============================================================================

CREATE TABLE queues (
    id                      UUID            PRIMARY KEY,
    project_id              UUID            NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                    VARCHAR(255)    NOT NULL,
    description             TEXT,
    priority                INT             NOT NULL DEFAULT 5
                                            CHECK (priority BETWEEN 1 AND 10),
    concurrency             INT             NOT NULL DEFAULT 10
                                            CHECK (concurrency > 0),
    max_retries             INT             NOT NULL DEFAULT 3
                                            CHECK (max_retries >= 0),
    retry_strategy          retry_strategy  NOT NULL DEFAULT 'exponential',
    retry_delay_sec         INT             NOT NULL DEFAULT 60
                                            CHECK (retry_delay_sec > 0),
    visibility_timeout_sec  INT             NOT NULL DEFAULT 300
                                            CHECK (visibility_timeout_sec > 0),
    job_timeout_sec         INT             NOT NULL DEFAULT 300
                                            CHECK (job_timeout_sec > 0),
    is_paused               BOOLEAN         NOT NULL DEFAULT FALSE,
    is_dlq                  BOOLEAN         NOT NULL DEFAULT FALSE,
    dlq_queue_id            UUID            REFERENCES queues(id) ON DELETE SET NULL,
    metadata                JSONB           NOT NULL DEFAULT '{}',
    created_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, name)
);

CREATE INDEX idx_queues_project_id ON queues (project_id);
CREATE INDEX idx_queues_priority   ON queues (project_id, priority) WHERE is_paused = FALSE;

-- =============================================================================
-- CRON JOB DEFINITIONS
-- =============================================================================

CREATE TABLE cron_jobs (
    id              UUID        PRIMARY KEY,
    queue_id        UUID        NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    cron_expr       VARCHAR(100) NOT NULL,  -- standard cron (5 or 6 fields)
    timezone        VARCHAR(100) NOT NULL DEFAULT 'UTC',
    job_type        VARCHAR(255) NOT NULL,
    payload         JSONB       NOT NULL DEFAULT '{}',
    max_retries     INT,                    -- NULL = inherit from queue
    retry_strategy  retry_strategy,        -- NULL = inherit from queue
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    last_fired_at   TIMESTAMPTZ,
    next_fire_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (queue_id, name)
);

CREATE INDEX idx_cron_jobs_queue_id     ON cron_jobs (queue_id);
CREATE INDEX idx_cron_jobs_next_fire_at ON cron_jobs (next_fire_at)
    WHERE is_active = TRUE;

-- =============================================================================
-- BATCHES
-- =============================================================================

CREATE TABLE batches (
    id              UUID        PRIMARY KEY,
    queue_id        UUID        NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    name            VARCHAR(255),
    description     TEXT,
    total_jobs      INT         NOT NULL DEFAULT 0,
    pending_count   INT         NOT NULL DEFAULT 0,
    running_count   INT         NOT NULL DEFAULT 0,
    succeeded_count INT         NOT NULL DEFAULT 0,
    failed_count    INT         NOT NULL DEFAULT 0,
    dead_count      INT         NOT NULL DEFAULT 0,
    status          batch_status NOT NULL DEFAULT 'pending',
    metadata        JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_batches_queue_id ON batches (queue_id);
CREATE INDEX idx_batches_status   ON batches (status) WHERE status NOT IN ('completed', 'cancelled');

-- =============================================================================
-- JOBS  (core table — heavily indexed for high-throughput dispatch)
-- =============================================================================

CREATE TABLE jobs (
    id              UUID        PRIMARY KEY,
    queue_id        UUID        NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    batch_id        UUID        REFERENCES batches(id) ON DELETE SET NULL,
    parent_job_id   UUID        REFERENCES jobs(id)   ON DELETE SET NULL,
    cron_job_id     UUID        REFERENCES cron_jobs(id) ON DELETE SET NULL,

    -- Job identity
    type            VARCHAR(255) NOT NULL,          -- handler identifier (e.g. "send_email")
    payload         JSONB       NOT NULL DEFAULT '{}',
    idempotency_key VARCHAR(255) UNIQUE,            -- optional, prevents duplicates

    -- Lifecycle
    status          job_status  NOT NULL DEFAULT 'pending',
    priority        INT         NOT NULL DEFAULT 5
                                CHECK (priority BETWEEN 1 AND 10),
    run_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),  -- when to execute

    -- Retry configuration (overrides queue defaults if set)
    max_retries     INT         NOT NULL DEFAULT 3,
    attempt_count   INT         NOT NULL DEFAULT 0,
    retry_strategy  retry_strategy NOT NULL DEFAULT 'exponential',
    retry_delay_sec INT         NOT NULL DEFAULT 60,
    last_error      TEXT,
    last_error_at   TIMESTAMPTZ,

    -- Execution tracking
    timeout_sec     INT         NOT NULL DEFAULT 300,
    claimed_by      UUID,                           -- worker.id that claimed this job
    claimed_at      TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,

    -- Metadata
    tags            TEXT[]      NOT NULL DEFAULT '{}',
    metadata        JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary dispatch index: queue + eligible statuses + ordering
-- SKIP LOCKED makes this highly concurrent with no contention
CREATE INDEX idx_jobs_dispatch ON jobs
    (queue_id, priority DESC, run_at ASC)
    WHERE status IN ('pending', 'scheduled');

-- For scheduler loop: find scheduled jobs due for promotion
CREATE INDEX idx_jobs_scheduled_due ON jobs (run_at)
    WHERE status = 'scheduled';

-- For reaper: find stale running jobs
CREATE INDEX idx_jobs_running_stale ON jobs (claimed_at, claimed_by)
    WHERE status = 'running';

-- For job listing APIs: by queue with status filter
CREATE INDEX idx_jobs_queue_status ON jobs (queue_id, status, created_at DESC);

-- For idempotency lookups
CREATE INDEX idx_jobs_idempotency ON jobs (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- For batch tracking
CREATE INDEX idx_jobs_batch_id ON jobs (batch_id) WHERE batch_id IS NOT NULL;

-- For cron lineage
CREATE INDEX idx_jobs_cron_job_id ON jobs (cron_job_id) WHERE cron_job_id IS NOT NULL;

-- =============================================================================
-- WORKERS
-- =============================================================================

CREATE TABLE workers (
    id                  UUID            PRIMARY KEY,
    hostname            VARCHAR(255)    NOT NULL,
    pid                 INT             NOT NULL,
    version             VARCHAR(50)     NOT NULL DEFAULT 'unknown',
    status              worker_status   NOT NULL DEFAULT 'active',
    concurrency         INT             NOT NULL,
    queues              TEXT[]          NOT NULL DEFAULT '{}',  -- queue names this worker handles
    last_heartbeat_at   TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    registered_at       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    metadata            JSONB           NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_workers_status          ON workers (status);
CREATE INDEX idx_workers_last_heartbeat  ON workers (last_heartbeat_at)
    WHERE status = 'active';

-- =============================================================================
-- JOB EXECUTIONS  (one row per attempt)
-- =============================================================================

CREATE TABLE job_executions (
    id              UUID            PRIMARY KEY,
    job_id          UUID            NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    worker_id       UUID            REFERENCES workers(id) ON DELETE SET NULL,
    attempt         INT             NOT NULL,
    status          execution_status NOT NULL DEFAULT 'started',
    started_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    duration_ms     INT,
    error_message   TEXT,
    stack_trace     TEXT,
    result          JSONB
);

CREATE INDEX idx_job_executions_job_id    ON job_executions (job_id);
CREATE INDEX idx_job_executions_worker_id ON job_executions (worker_id);
CREATE INDEX idx_job_executions_started   ON job_executions (job_id, attempt);

-- =============================================================================
-- EXECUTION LOGS  (structured log lines per execution)
-- =============================================================================

CREATE TABLE execution_logs (
    id              UUID        PRIMARY KEY,
    execution_id    UUID        NOT NULL REFERENCES job_executions(id) ON DELETE CASCADE,
    level           log_level   NOT NULL DEFAULT 'info',
    message         TEXT        NOT NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}',
    logged_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_execution_logs_execution_id ON execution_logs (execution_id, logged_at);

-- =============================================================================
-- METRICS  (pre-aggregated time-series for dashboard)
-- =============================================================================

CREATE TABLE queue_metrics (
    id              UUID        PRIMARY KEY,
    queue_id        UUID        NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    window_start    TIMESTAMPTZ NOT NULL,
    window_end      TIMESTAMPTZ NOT NULL,
    jobs_enqueued   INT         NOT NULL DEFAULT 0,
    jobs_succeeded  INT         NOT NULL DEFAULT 0,
    jobs_failed     INT         NOT NULL DEFAULT 0,
    jobs_dead       INT         NOT NULL DEFAULT 0,
    jobs_cancelled  INT         NOT NULL DEFAULT 0,
    avg_wait_ms     INT         NOT NULL DEFAULT 0,  -- time from enqueue to start
    avg_duration_ms INT         NOT NULL DEFAULT 0,  -- execution duration
    p95_duration_ms INT         NOT NULL DEFAULT 0,
    p99_duration_ms INT         NOT NULL DEFAULT 0,
    UNIQUE (queue_id, window_start)
);

CREATE INDEX idx_queue_metrics_queue_window ON queue_metrics (queue_id, window_start DESC);

-- =============================================================================
-- UPDATED_AT TRIGGERS (auto-maintain updated_at on all major tables)
-- =============================================================================

CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to all tables with updated_at
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users', 'organizations', 'projects', 'queues', 'cron_jobs',
        'batches', 'jobs'
    ] LOOP
        EXECUTE format(
            'CREATE TRIGGER trg_%I_updated_at
             BEFORE UPDATE ON %I
             FOR EACH ROW EXECUTE FUNCTION trigger_set_updated_at()',
            t, t
        );
    END LOOP;
END;
$$;
