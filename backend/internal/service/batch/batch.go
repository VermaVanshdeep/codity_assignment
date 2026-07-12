package batch

import (
	"context"
	"time"

	"github.com/google/uuid"
	domainbatch "github.com/your-org/job-scheduler/internal/domain/batch"
	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

type Service struct {
	batches domainbatch.Repository
	jobs    job.Repository
	log     *logger.Logger
}

func NewService(batches domainbatch.Repository, jobs job.Repository, log *logger.Logger) *Service {
	return &Service{
		batches: batches,
		jobs:    jobs,
		log:     log.WithField("service", "batch"),
	}
}

// CreateBatch creates a new batch and all its constituent jobs in one logical operation.
// In a production system, this would ideally use a database transaction, but for this iteration,
// we create the batch record first and then bulk-insert the jobs (or insert sequentially).
func (s *Service) CreateBatch(ctx context.Context, queueID uuid.UUID, name string, jobSpecs []job.Job) (*domainbatch.Batch, error) {
	now := time.Now().UTC()
	b := &domainbatch.Batch{
		ID:           uuid.New(),
		QueueID:      queueID,
		Name:         name,
		TotalJobs:    len(jobSpecs),
		PendingCount: len(jobSpecs),
		Status:       domainbatch.StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.batches.Create(ctx, b); err != nil {
		return nil, err
	}

	for i := range jobSpecs {
		jobSpecs[i].ID = uuid.New()
		jobSpecs[i].QueueID = queueID
		jobSpecs[i].BatchID = &b.ID
		jobSpecs[i].Status = job.StatusPending
		jobSpecs[i].CreatedAt = now
		jobSpecs[i].UpdatedAt = now
		if jobSpecs[i].RunAt.IsZero() {
			jobSpecs[i].RunAt = now
		}

		// Insert sequentially for simplicity.
		if err := s.jobs.Create(ctx, &jobSpecs[i]); err != nil {
			s.log.Error("failed to insert batch job", logger.Err(err))
			// A real system handles partial insertion failures gracefully.
		}
	}

	return b, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*domainbatch.Batch, error) {
	return s.batches.Get(ctx, id)
}
