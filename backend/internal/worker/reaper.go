package worker

import (
	"context"
	"time"

	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Reaper runs a background loop to find and reclaim jobs that are stuck
// in the 'running' state because their worker crashed or lost network connection.
type Reaper struct {
	jobs              job.Repository
	pollInterval      time.Duration
	visibilityTimeout time.Duration
	log               *logger.Logger
}

// NewReaper creates a new Reaper.
func NewReaper(
	jobs job.Repository,
	pollInterval time.Duration,
	visibilityTimeout time.Duration,
	log *logger.Logger,
) *Reaper {
	return &Reaper{
		jobs:              jobs,
		pollInterval:      pollInterval,
		visibilityTimeout: visibilityTimeout,
		log:               log.WithField("component", "reaper"),
	}
}

// Start runs the reaper loop until context is canceled.
func (r *Reaper) Start(ctx context.Context) {
	r.log.Info("reaper started",
		logger.String("poll_interval", r.pollInterval.String()),
		logger.String("visibility_timeout", r.visibilityTimeout.String()),
	)

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("reaper stopping")
			return
		case <-ticker.C:
			r.reclaim(ctx)
		}
	}
}

func (r *Reaper) reclaim(ctx context.Context) {
	count, err := r.jobs.ReclaimStale(ctx, r.visibilityTimeout)
	if err != nil {
		r.log.Error("failed to reclaim stale jobs", logger.Err(err))
		return
	}
	if count > 0 {
		r.log.Warn("reclaimed stale jobs", logger.Int64("count", count))
	}
}
