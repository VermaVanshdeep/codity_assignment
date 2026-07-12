package worker

import (
	"context"
	"time"

	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Scheduler runs a background loop to promote jobs from 'scheduled' to 'pending'
// when their run_at time is reached.
type Scheduler struct {
	jobs         job.Repository
	pollInterval time.Duration
	log          *logger.Logger
}

// NewScheduler creates a new Scheduler.
func NewScheduler(jobs job.Repository, pollInterval time.Duration, log *logger.Logger) *Scheduler {
	return &Scheduler{
		jobs:         jobs,
		pollInterval: pollInterval,
		log:          log.WithField("component", "scheduler"),
	}
}

// Start runs the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info("scheduler started", logger.String("poll_interval", s.pollInterval.String()))
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopping")
			return
		case <-ticker.C:
			s.promote(ctx)
		}
	}
}

func (s *Scheduler) promote(ctx context.Context) {
	count, err := s.jobs.PromoteScheduled(ctx)
	if err != nil {
		s.log.Error("failed to promote scheduled jobs", logger.Err(err))
		return
	}
	if count > 0 {
		s.log.Debug("promoted scheduled jobs", logger.Int64("count", count))
	}
}
