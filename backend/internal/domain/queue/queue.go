// Package queue defines the Queue domain entity and its repository contract.
// A Queue belongs to a Project and is the unit of scheduling configuration.
// All retry, concurrency, priority, and timeout settings live here.
package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Queue is the scheduling configuration unit within a project.
type Queue struct {
	ID                   uuid.UUID
	ProjectID            uuid.UUID
	Name                 string
	Description          string
	Priority             int // 1 (highest) – 10 (lowest), default 5
	Concurrency          int // max parallel running jobs
	MaxRetries           int
	RetryStrategy        string // fixed | linear | exponential
	RetryDelaySec        int
	VisibilityTimeoutSec int // max seconds a job can be in 'running' state
	JobTimeoutSec        int // per-execution hard timeout
	IsPaused             bool
	IsDLQ                bool       // is this queue acting as a Dead Letter Queue?
	DLQQueueID           *uuid.UUID // points to associated DLQ
	Metadata             map[string]any
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Stats is a real-time snapshot of a queue's job counts.
type Stats struct {
	QueueID        uuid.UUID `json:"queue_id"`
	PendingCount   int64     `json:"pending_count"`
	ScheduledCount int64     `json:"scheduled_count"`
	RunningCount   int64     `json:"running_count"`
	SucceededCount int64     `json:"succeeded_count"`
	FailedCount    int64     `json:"failed_count"`
	DeadCount      int64     `json:"dead_count"`
	CancelledCount int64     `json:"cancelled_count"`
}

// Repository defines the data access contract for queue persistence.
type Repository interface {
	Create(ctx context.Context, q *Queue) error
	GetByID(ctx context.Context, id uuid.UUID) (*Queue, error)
	GetByName(ctx context.Context, projectID uuid.UUID, name string) (*Queue, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]*Queue, error)
	// ListActivePrioritized returns non-paused queues ordered by priority ASC (1=highest).
	ListActivePrioritized(ctx context.Context) ([]*Queue, error)
	Update(ctx context.Context, q *Queue) error
	Delete(ctx context.Context, id uuid.UUID) error
	SetPaused(ctx context.Context, id uuid.UUID, paused bool) error
	GetStats(ctx context.Context, id uuid.UUID) (*Stats, error)
}
