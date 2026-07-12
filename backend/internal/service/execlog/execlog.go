package execlog

import (
	"context"

	"github.com/google/uuid"
	domainexeclog "github.com/your-org/job-scheduler/internal/domain/execlog"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

type Service struct {
	repo domainexeclog.Repository
	log  *logger.Logger
}

func NewService(repo domainexeclog.Repository, log *logger.Logger) *Service {
	return &Service{repo: repo, log: log.WithField("service", "execlog")}
}

func (s *Service) ListExecutions(ctx context.Context, jobID uuid.UUID) ([]domainexeclog.JobExecution, error) {
	return s.repo.ListExecutionsByJob(ctx, jobID)
}

func (s *Service) ListLogs(ctx context.Context, jobID uuid.UUID) ([]domainexeclog.ExecutionLog, error) {
	return s.repo.ListLogsByJob(ctx, jobID)
}
