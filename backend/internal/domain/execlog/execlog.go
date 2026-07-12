package execlog

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// ExecutionLog represents a single log line emitted during a job's execution.
type ExecutionLog struct {
	ID          uuid.UUID
	ExecutionID uuid.UUID // links to job_executions table
	JobID       uuid.UUID
	Level       LogLevel
	Message     string
	Metadata    map[string]any
	LoggedAt    time.Time
}

// JobExecution represents a single attempt to run a job.
type JobExecution struct {
	ID           uuid.UUID
	JobID        uuid.UUID
	WorkerID     uuid.UUID
	Attempt      int
	Status       string
	StartedAt    time.Time
	CompletedAt  *time.Time
	DurationMs   *int
	ErrorMessage *string
	StackTrace   *string
	Result       map[string]any
}

type Repository interface {
	// Job Execution
	CreateExecution(ctx context.Context, e *JobExecution) error
	UpdateExecution(ctx context.Context, e *JobExecution) error
	ListExecutionsByJob(ctx context.Context, jobID uuid.UUID) ([]JobExecution, error)

	// Logs
	AddLogs(ctx context.Context, logs []ExecutionLog) error
	ListLogsByExecution(ctx context.Context, executionID uuid.UUID) ([]ExecutionLog, error)
	ListLogsByJob(ctx context.Context, jobID uuid.UUID) ([]ExecutionLog, error)
}
