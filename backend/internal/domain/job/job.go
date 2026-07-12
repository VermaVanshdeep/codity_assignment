// Package job defines the Job domain entity and its complete lifecycle state machine.
// This is the core entity of the entire system.
package job

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Status represents a job's position in its lifecycle.
// Transitions are strictly controlled by the dispatcher, worker, and retry engine.
type Status string

const (
	// StatusPending: ready to be claimed by a worker.
	StatusPending Status = "pending"
	// StatusScheduled: has a future run_at, waiting for the scheduler to promote.
	StatusScheduled Status = "scheduled"
	// StatusRunning: claimed by a worker and currently executing.
	StatusRunning Status = "running"
	// StatusSucceeded: executed successfully.
	StatusSucceeded Status = "succeeded"
	// StatusFailed: execution failed, will be retried if attempts remain.
	StatusFailed Status = "failed"
	// StatusDead: exhausted all retries — moved to DLQ.
	StatusDead Status = "dead"
	// StatusCancelled: cancelled by user before execution.
	StatusCancelled Status = "cancelled"
)

// Job is the core scheduling entity.
type Job struct {
	ID             uuid.UUID
	QueueID        uuid.UUID
	BatchID        *uuid.UUID
	ParentJobID    *uuid.UUID // for workflow dependencies
	CronJobID      *uuid.UUID // set when spawned from a cron definition
	Type           string     // handler identifier, e.g. "email.send"
	Payload        map[string]any
	IdempotencyKey *string
	Status         Status
	Priority       int
	RunAt          time.Time // when to execute; past = immediate
	MaxRetries     int
	AttemptCount   int
	RetryStrategy  string
	RetryDelaySec  int
	LastError      *string
	LastErrorAt    *time.Time
	TimeoutSec     int
	ClaimedBy      *uuid.UUID
	ClaimedAt      *time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Tags           []string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsTerminal returns true if the job is in a final, non-recoverable state.
func (j *Job) IsTerminal() bool {
	return j.Status == StatusSucceeded ||
		j.Status == StatusDead ||
		j.Status == StatusCancelled
}

// CanRetry returns true if the job has remaining retry attempts.
func (j *Job) CanRetry() bool {
	return j.AttemptCount < j.MaxRetries
}

// CreateRequest holds all fields needed to enqueue a new job.
type CreateRequest struct {
	Type           string
	Payload        map[string]any
	Priority       *int
	RunAt          *time.Time // nil = immediate
	MaxRetries     *int
	RetryStrategy  *string
	RetryDelaySec  *int
	TimeoutSec     *int
	IdempotencyKey *string
	Tags           []string
	Metadata       map[string]any
	BatchID        *uuid.UUID
	ParentJobID    *uuid.UUID
	CronJobID      *uuid.UUID
}

// ListFilter contains filtering and pagination options for job queries.
type ListFilter struct {
	Status    *Status
	Type      *string
	Tags      []string
	RunAfter  *time.Time
	RunBefore *time.Time
	Limit     int
	Offset    int
}

// Repository defines the data access contract for job persistence.
type Repository interface {
	Create(ctx context.Context, j *Job) error
	GetByID(ctx context.Context, id uuid.UUID) (*Job, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*Job, error)
	List(ctx context.Context, queueID uuid.UUID, filter ListFilter) ([]*Job, int64, error)

	// ClaimBatch atomically claims up to batchSize eligible jobs for the given worker.
	// Uses SELECT … FOR UPDATE SKIP LOCKED to ensure no two workers claim the same job.
	ClaimBatch(ctx context.Context, queueID uuid.UUID, workerID uuid.UUID, batchSize int, concurrencyLimit int) ([]*Job, error)

	UpdateStatus(ctx context.Context, id uuid.UUID, status Status) error
	MarkSucceeded(ctx context.Context, id uuid.UUID, completedAt time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, failedAt time.Time) error
	MarkDead(ctx context.Context, id uuid.UUID, errMsg string, failedAt time.Time) error
	ScheduleRetry(ctx context.Context, id uuid.UUID, runAt time.Time, attemptCount int) error
	Cancel(ctx context.Context, id uuid.UUID) error

	// PromoteScheduled promotes jobs whose run_at <= now from scheduled → pending.
	PromoteScheduled(ctx context.Context) (int64, error)

	// ReclaimStale re-queues jobs held by dead/stale workers.
	ReclaimStale(ctx context.Context, visibilityTimeout time.Duration) (int64, error)
}
