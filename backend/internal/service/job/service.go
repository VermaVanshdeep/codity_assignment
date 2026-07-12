// Package job provides business logic for job creation, cancellation, and inspection.
package job

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/domain/queue"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// EnqueueRequest is the API payload to submit a new job.
type EnqueueRequest struct {
	Type           string         `json:"type"            validate:"required,min=1,max=255"`
	Payload        map[string]any `json:"payload"`
	Priority       *int           `json:"priority"` // nil = inherit from queue
	RunAt          *time.Time     `json:"run_at"`   // nil = immediate
	MaxRetries     *int           `json:"max_retries"`
	RetryStrategy  *string        `json:"retry_strategy"`
	RetryDelaySec  *int           `json:"retry_delay_sec"`
	TimeoutSec     *int           `json:"timeout_sec"`
	IdempotencyKey *string        `json:"idempotency_key"`
	Tags           []string       `json:"tags"`
	Metadata       map[string]any `json:"metadata"`
}

// JobResponse is the API representation of a job.
type JobResponse struct {
	ID             uuid.UUID      `json:"id"`
	QueueID        uuid.UUID      `json:"queue_id"`
	BatchID        *uuid.UUID     `json:"batch_id,omitempty"`
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload"`
	Status         job.Status     `json:"status"`
	Priority       int            `json:"priority"`
	RunAt          time.Time      `json:"run_at"`
	MaxRetries     int            `json:"max_retries"`
	AttemptCount   int            `json:"attempt_count"`
	RetryStrategy  string         `json:"retry_strategy"`
	LastError      *string        `json:"last_error,omitempty"`
	TimeoutSec     int            `json:"timeout_sec"`
	IdempotencyKey *string        `json:"idempotency_key,omitempty"`
	Tags           []string       `json:"tags"`
	ClaimedBy      *uuid.UUID     `json:"claimed_by,omitempty"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// ListResponse wraps a page of jobs with pagination metadata.
type ListResponse struct {
	Jobs   []*JobResponse `json:"jobs"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// ─── Interface ─────────────────────────────────────────────────────────────────

// Service defines the job management operations.
type Service interface {
	Enqueue(ctx context.Context, queueID uuid.UUID, req EnqueueRequest) (*JobResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*JobResponse, error)
	List(ctx context.Context, queueID uuid.UUID, filter job.ListFilter) (*ListResponse, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	Retry(ctx context.Context, id uuid.UUID) error // manually retry a dead/failed job
}

// ─── Implementation ────────────────────────────────────────────────────────────

type service struct {
	jobs   job.Repository
	queues queue.Repository
	log    *logger.Logger
}

// NewService creates a new job Service.
func NewService(jobs job.Repository, queues queue.Repository, log *logger.Logger) Service {
	return &service{jobs: jobs, queues: queues, log: log.WithField("service", "job")}
}

// Enqueue submits a new job to the specified queue.
// It respects the queue's defaults for retry and timeout settings,
// and enforces idempotency via the optional idempotency_key.
func (s *service) Enqueue(ctx context.Context, queueID uuid.UUID, req EnqueueRequest) (*JobResponse, error) {
	// Check idempotency key — return existing job if already submitted.
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		existing, err := s.jobs.GetByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return toJobResponse(existing), nil // idempotent — same job returned
		}
		if !apperrors.IsNotFound(err) {
			return nil, err
		}
	}

	// Fetch queue to inherit defaults.
	q, err := s.queues.GetByID(ctx, queueID)
	if err != nil {
		return nil, err
	}
	if q.IsPaused {
		return nil, apperrors.Conflict("queue is paused — jobs cannot be enqueued")
	}

	now := time.Now().UTC()
	j := &job.Job{
		ID:            uuid.New(),
		QueueID:       queueID,
		Type:          req.Type,
		Payload:       req.Payload,
		Status:        job.StatusPending,
		Priority:      q.Priority,
		RunAt:         now,
		MaxRetries:    q.MaxRetries,
		AttemptCount:  0,
		RetryStrategy: q.RetryStrategy,
		RetryDelaySec: q.RetryDelaySec,
		TimeoutSec:    q.JobTimeoutSec,
		Tags:          req.Tags,
		Metadata:      req.Metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Apply overrides from request.
	if req.Priority != nil {
		j.Priority = *req.Priority
	}
	if req.MaxRetries != nil {
		j.MaxRetries = *req.MaxRetries
	}
	if req.RetryStrategy != nil {
		j.RetryStrategy = *req.RetryStrategy
	}
	if req.RetryDelaySec != nil {
		j.RetryDelaySec = *req.RetryDelaySec
	}
	if req.TimeoutSec != nil {
		j.TimeoutSec = *req.TimeoutSec
	}
	if req.IdempotencyKey != nil {
		j.IdempotencyKey = req.IdempotencyKey
	}

	// Delayed job: set future run_at and scheduled status.
	if req.RunAt != nil && req.RunAt.After(now) {
		j.RunAt = req.RunAt.UTC()
		j.Status = job.StatusScheduled
	}

	if err := s.jobs.Create(ctx, j); err != nil {
		return nil, err
	}
	s.log.Info("job enqueued",
		logger.String("job_id", j.ID.String()),
		logger.String("type", j.Type),
		logger.String("status", string(j.Status)),
	)
	return toJobResponse(j), nil
}

// GetByID retrieves a job by its ID.
func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*JobResponse, error) {
	j, err := s.jobs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toJobResponse(j), nil
}

// List returns a paginated list of jobs for a queue.
func (s *service) List(ctx context.Context, queueID uuid.UUID, filter job.ListFilter) (*ListResponse, error) {
	jobs, total, err := s.jobs.List(ctx, queueID, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*JobResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, toJobResponse(j))
	}
	return &ListResponse{Jobs: out, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// Cancel cancels a pending or scheduled job.
func (s *service) Cancel(ctx context.Context, id uuid.UUID) error {
	return s.jobs.Cancel(ctx, id)
}

// Retry manually re-enqueues a failed or dead job.
func (s *service) Retry(ctx context.Context, id uuid.UUID) error {
	j, err := s.jobs.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if j.Status != job.StatusFailed && j.Status != job.StatusDead {
		return apperrors.Conflict("only failed or dead jobs can be manually retried")
	}
	return s.jobs.ScheduleRetry(ctx, id, time.Now().UTC(), j.AttemptCount)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func toJobResponse(j *job.Job) *JobResponse {
	return &JobResponse{
		ID: j.ID, QueueID: j.QueueID, BatchID: j.BatchID, Type: j.Type,
		Payload: j.Payload, Status: j.Status, Priority: j.Priority, RunAt: j.RunAt,
		MaxRetries: j.MaxRetries, AttemptCount: j.AttemptCount,
		RetryStrategy: j.RetryStrategy, LastError: j.LastError,
		TimeoutSec: j.TimeoutSec, IdempotencyKey: j.IdempotencyKey, Tags: j.Tags,
		ClaimedBy: j.ClaimedBy, StartedAt: j.StartedAt, CompletedAt: j.CompletedAt,
		CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
	}
}
