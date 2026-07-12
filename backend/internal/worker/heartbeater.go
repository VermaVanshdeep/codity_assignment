package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/worker"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

type Heartbeater struct {
	workerID uuid.UUID
	repo     worker.Repository
	interval time.Duration
	log      *logger.Logger
}

func NewHeartbeater(workerID uuid.UUID, repo worker.Repository, interval time.Duration, log *logger.Logger) *Heartbeater {
	return &Heartbeater{
		workerID: workerID,
		repo:     repo,
		interval: interval,
		log:      log.WithField("component", "heartbeater"),
	}
}

func (h *Heartbeater) Start(ctx context.Context) {
	h.log.Info("starting heartbeater", logger.Duration("interval", h.interval.Milliseconds()))
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.log.Info("heartbeater stopping")
			return
		case <-ticker.C:
			if err := h.repo.Heartbeat(ctx, h.workerID); err != nil {
				h.log.Error("failed to heartbeat", logger.Err(err))
			}
		}
	}
}
