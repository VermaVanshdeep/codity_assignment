package metrics

import (
	"context"
	"time"

	"github.com/google/uuid"
	domainmetrics "github.com/your-org/job-scheduler/internal/domain/metrics"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

type Service struct {
	repo domainmetrics.Repository
	log  *logger.Logger
}

func NewService(repo domainmetrics.Repository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log.WithField("service", "metrics")}
}

func (s *Service) GetQueueMetrics(ctx context.Context, queueID uuid.UUID, since time.Time) ([]domainmetrics.QueueMetricsSnapshot, error) {
	return s.repo.GetSnapshots(ctx, queueID, since)
}

func (s *Service) GetSystemMetrics(ctx context.Context) (*domainmetrics.SystemMetrics, error) {
	return s.repo.GetSystemMetrics(ctx)
}
