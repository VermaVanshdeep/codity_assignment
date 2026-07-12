package batch

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending        Status = "pending"
	StatusRunning        Status = "running"
	StatusCompleted      Status = "completed"
	StatusPartialFailure Status = "partial_failure"
)

type Batch struct {
	ID             uuid.UUID
	QueueID        uuid.UUID
	Name           string
	TotalJobs      int
	PendingCount   int
	SucceededCount int
	FailedCount    int
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Repository interface {
	Create(ctx context.Context, b *Batch) error
	Get(ctx context.Context, id uuid.UUID) (*Batch, error)
	// IncrementCounters updates the succeeded/failed counts. It's usually called by a webhook or job completion hook.
	IncrementCounters(ctx context.Context, id uuid.UUID, succeeded int, failed int) error
}
