package cron

import (
	"context"
	"time"

	"github.com/google/uuid"
	domaincron "github.com/your-org/job-scheduler/internal/domain/cron"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

type Service struct {
	crons domaincron.Repository
	log   *logger.Logger
}

func NewService(crons domaincron.Repository, log *logger.Logger) *Service {
	return &Service{crons: crons, log: log.WithField("service", "cron")}
}

func (s *Service) Create(ctx context.Context, c *domaincron.CronJob) error {
	now := time.Now().UTC()
	c.ID = uuid.New()
	c.CreatedAt = now
	c.UpdatedAt = now
	return s.crons.Create(ctx, c)
}

func (s *Service) ListByQueueID(ctx context.Context, queueID uuid.UUID) ([]*domaincron.CronJob, error) {
	return s.crons.ListByQueueID(ctx, queueID)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.crons.Delete(ctx, id)
}
