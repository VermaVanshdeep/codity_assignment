// Package cron defines the CronJob domain entity for recurring job scheduling.
package cron

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CronJob is a recurring job definition that fires on a schedule.
type CronJob struct {
	ID            uuid.UUID
	QueueID       uuid.UUID
	Name          string
	Description   string
	CronExpr      string // standard 5-field cron expression
	Timezone      string
	JobType       string
	Payload       map[string]any
	MaxRetries    *int    // nil = inherit from queue
	RetryStrategy *string // nil = inherit from queue
	IsActive      bool
	LastFiredAt   *time.Time
	NextFireAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repository defines the data access contract for cron job persistence.
type Repository interface {
	Create(ctx context.Context, c *CronJob) error
	GetByID(ctx context.Context, id uuid.UUID) (*CronJob, error)
	ListByQueueID(ctx context.Context, queueID uuid.UUID) ([]*CronJob, error)
	// ListDue returns active cron jobs whose next_fire_at <= now.
	ListDue(ctx context.Context, now time.Time) ([]*CronJob, error)
	Update(ctx context.Context, c *CronJob) error
	UpdateFireTimes(ctx context.Context, id uuid.UUID, lastFiredAt, nextFireAt time.Time) error
	SetActive(ctx context.Context, id uuid.UUID, active bool) error
	Delete(ctx context.Context, id uuid.UUID) error
}
