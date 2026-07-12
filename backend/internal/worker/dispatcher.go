package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/domain/queue"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Dispatcher polls active queues for pending jobs and dispatches them to the worker pool.
type Dispatcher struct {
	workerID     uuid.UUID
	queues       queue.Repository
	jobs         job.Repository
	pool         *Pool
	pollInterval time.Duration
	batchSize    int
	log          *logger.Logger
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(
	workerID uuid.UUID,
	queues queue.Repository,
	jobs job.Repository,
	pool *Pool,
	pollInterval time.Duration,
	batchSize int,
	log *logger.Logger,
) *Dispatcher {
	return &Dispatcher{
		workerID:     workerID,
		queues:       queues,
		jobs:         jobs,
		pool:         pool,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		log:          log.WithField("component", "dispatcher"),
	}
}

// Start runs the dispatcher loop until the context is canceled.
func (d *Dispatcher) Start(ctx context.Context) {
	d.log.Info("dispatcher started",
		logger.String("poll_interval", d.pollInterval.String()),
		logger.Int("batch_size", d.batchSize),
	)

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Info("dispatcher stopping (context canceled)")
			return
		case <-ticker.C:
			d.dispatchAll(ctx)
		}
	}
}

// dispatchAll loops through all active queues in priority order and attempts to claim jobs.
func (d *Dispatcher) dispatchAll(ctx context.Context) {
	// 1. Get all active queues, ordered by priority (1 is highest).
	activeQueues, err := d.queues.ListActivePrioritized(ctx)
	if err != nil {
		d.log.Error("failed to list active queues", logger.Err(err))
		return
	}

	if len(activeQueues) == 0 {
		return
	}

	// 2. For each queue, try to claim a batch of jobs if the worker pool has capacity.
	for _, q := range activeQueues {
		// Stop checking queues if our local worker pool is full.
		if d.pool.AvailableWorkers() <= 0 {
			break
		}
		d.dispatchQueue(ctx, q)
	}
}

// dispatchQueue attempts to claim jobs for a single queue and submit them to the pool.
func (d *Dispatcher) dispatchQueue(ctx context.Context, q *queue.Queue) {
	available := d.pool.AvailableWorkers()
	if available <= 0 {
		return
	}

	claimSize := d.batchSize
	if available < claimSize {
		claimSize = available
	}

	// Claim atomic batch using SKIP LOCKED.
	claimed, err := d.jobs.ClaimBatch(ctx, q.ID, d.workerID, claimSize, q.Concurrency)
	if err != nil {
		d.log.Error("failed to claim batch",
			logger.String("queue_id", q.ID.String()),
			logger.Err(err),
		)
		return
	}

	if len(claimed) > 0 {
		d.log.Debug("claimed jobs",
			logger.String("queue_id", q.ID.String()),
			logger.Int("count", len(claimed)),
		)
	}

	// Dispatch each claimed job to the local pool.
	for _, j := range claimed {
		d.pool.Submit(j)
	}
}
