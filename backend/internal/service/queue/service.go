// Package queue provides business logic for Queue lifecycle management.
package queue

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/queue"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// CreateRequest contains fields to configure a new queue.
type CreateRequest struct {
	Name                 string         `json:"name"                   validate:"required,min=1,max=100"`
	Description          string         `json:"description"            validate:"max=500"`
	Priority             int            `json:"priority"               validate:"gte=1,lte=10"`
	Concurrency          int            `json:"concurrency"            validate:"gte=1,lte=1000"`
	MaxRetries           int            `json:"max_retries"            validate:"gte=0,lte=100"`
	RetryStrategy        string         `json:"retry_strategy"         validate:"oneof=fixed linear exponential"`
	RetryDelaySec        int            `json:"retry_delay_sec"        validate:"gte=1"`
	VisibilityTimeoutSec int            `json:"visibility_timeout_sec" validate:"gte=10"`
	JobTimeoutSec        int            `json:"job_timeout_sec"        validate:"gte=1"`
	DLQQueueID           *uuid.UUID     `json:"dlq_queue_id"`
	Metadata             map[string]any `json:"metadata"`
}

// UpdateRequest contains mutable queue configuration fields.
type UpdateRequest struct {
	Description          string         `json:"description"            validate:"max=500"`
	Priority             int            `json:"priority"               validate:"gte=1,lte=10"`
	Concurrency          int            `json:"concurrency"            validate:"gte=1,lte=1000"`
	MaxRetries           int            `json:"max_retries"            validate:"gte=0,lte=100"`
	RetryStrategy        string         `json:"retry_strategy"         validate:"oneof=fixed linear exponential"`
	RetryDelaySec        int            `json:"retry_delay_sec"        validate:"gte=1"`
	VisibilityTimeoutSec int            `json:"visibility_timeout_sec" validate:"gte=10"`
	JobTimeoutSec        int            `json:"job_timeout_sec"        validate:"gte=1"`
	DLQQueueID           *uuid.UUID     `json:"dlq_queue_id"`
	Metadata             map[string]any `json:"metadata"`
}

// QueueResponse is the API representation of a queue.
type QueueResponse struct {
	ID                   uuid.UUID      `json:"id"`
	ProjectID            uuid.UUID      `json:"project_id"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	Priority             int            `json:"priority"`
	Concurrency          int            `json:"concurrency"`
	MaxRetries           int            `json:"max_retries"`
	RetryStrategy        string         `json:"retry_strategy"`
	RetryDelaySec        int            `json:"retry_delay_sec"`
	VisibilityTimeoutSec int            `json:"visibility_timeout_sec"`
	JobTimeoutSec        int            `json:"job_timeout_sec"`
	IsPaused             bool           `json:"is_paused"`
	IsDLQ                bool           `json:"is_dlq"`
	DLQQueueID           *uuid.UUID     `json:"dlq_queue_id,omitempty"`
	Metadata             map[string]any `json:"metadata"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// ─── Interface ─────────────────────────────────────────────────────────────────

// Service defines queue management operations.
type Service interface {
	Create(ctx context.Context, projectID uuid.UUID, req CreateRequest) (*QueueResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*QueueResponse, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*QueueResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*QueueResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Pause(ctx context.Context, id uuid.UUID) error
	Resume(ctx context.Context, id uuid.UUID) error
	GetStats(ctx context.Context, id uuid.UUID) (*queue.Stats, error)
}

// ─── Implementation ────────────────────────────────────────────────────────────

type service struct {
	queues queue.Repository
	log    *logger.Logger
}

// NewService creates a new queue Service.
func NewService(queues queue.Repository, log *logger.Logger) Service {
	return &service{queues: queues, log: log.WithField("service", "queue")}
}

func (s *service) Create(ctx context.Context, projectID uuid.UUID, req CreateRequest) (*QueueResponse, error) {
	setDefaults(&req)
	now := time.Now().UTC()
	q := &queue.Queue{
		ID:                   uuid.New(),
		ProjectID:            projectID,
		Name:                 strings.TrimSpace(req.Name),
		Description:          strings.TrimSpace(req.Description),
		Priority:             req.Priority,
		Concurrency:          req.Concurrency,
		MaxRetries:           req.MaxRetries,
		RetryStrategy:        req.RetryStrategy,
		RetryDelaySec:        req.RetryDelaySec,
		VisibilityTimeoutSec: req.VisibilityTimeoutSec,
		JobTimeoutSec:        req.JobTimeoutSec,
		DLQQueueID:           req.DLQQueueID,
		Metadata:             req.Metadata,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.queues.Create(ctx, q); err != nil {
		return nil, err
	}
	s.log.Info("queue created", logger.String("id", q.ID.String()), logger.String("name", q.Name))
	return toQueueResponse(q), nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*QueueResponse, error) {
	q, err := s.queues.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toQueueResponse(q), nil
}

func (s *service) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*QueueResponse, error) {
	queues, err := s.queues.ListByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*QueueResponse, 0, len(queues))
	for _, q := range queues {
		out = append(out, toQueueResponse(q))
	}
	return out, nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*QueueResponse, error) {
	q, err := s.queues.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	q.Description = strings.TrimSpace(req.Description)
	q.Priority = req.Priority
	q.Concurrency = req.Concurrency
	q.MaxRetries = req.MaxRetries
	q.RetryStrategy = req.RetryStrategy
	q.RetryDelaySec = req.RetryDelaySec
	q.VisibilityTimeoutSec = req.VisibilityTimeoutSec
	q.JobTimeoutSec = req.JobTimeoutSec
	q.DLQQueueID = req.DLQQueueID
	q.Metadata = req.Metadata
	q.UpdatedAt = time.Now().UTC()

	if err := s.queues.Update(ctx, q); err != nil {
		return nil, err
	}
	return toQueueResponse(q), nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.queues.Delete(ctx, id)
}

func (s *service) Pause(ctx context.Context, id uuid.UUID) error {
	q, err := s.queues.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if q.IsPaused {
		return apperrors.Conflict("queue is already paused")
	}
	return s.queues.SetPaused(ctx, id, true)
}

func (s *service) Resume(ctx context.Context, id uuid.UUID) error {
	q, err := s.queues.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !q.IsPaused {
		return apperrors.Conflict("queue is not paused")
	}
	return s.queues.SetPaused(ctx, id, false)
}

func (s *service) GetStats(ctx context.Context, id uuid.UUID) (*queue.Stats, error) {
	return s.queues.GetStats(ctx, id)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func setDefaults(req *CreateRequest) {
	if req.Priority == 0 {
		req.Priority = 5
	}
	if req.Concurrency == 0 {
		req.Concurrency = 10
	}
	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}
	if req.RetryStrategy == "" {
		req.RetryStrategy = "exponential"
	}
	if req.RetryDelaySec == 0 {
		req.RetryDelaySec = 60
	}
	if req.VisibilityTimeoutSec == 0 {
		req.VisibilityTimeoutSec = 300
	}
	if req.JobTimeoutSec == 0 {
		req.JobTimeoutSec = 300
	}
}

func toQueueResponse(q *queue.Queue) *QueueResponse {
	return &QueueResponse{
		ID: q.ID, ProjectID: q.ProjectID, Name: q.Name, Description: q.Description,
		Priority: q.Priority, Concurrency: q.Concurrency, MaxRetries: q.MaxRetries,
		RetryStrategy: q.RetryStrategy, RetryDelaySec: q.RetryDelaySec,
		VisibilityTimeoutSec: q.VisibilityTimeoutSec, JobTimeoutSec: q.JobTimeoutSec,
		IsPaused: q.IsPaused, IsDLQ: q.IsDLQ, DLQQueueID: q.DLQQueueID,
		Metadata: q.Metadata, CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt,
	}
}
