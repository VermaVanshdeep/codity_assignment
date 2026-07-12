package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/platform/logger"
	retrysvc "github.com/your-org/job-scheduler/pkg/retry"
)

// Executor runs a single job and handles its success/failure lifecycle.
// It applies the per-job timeout, calls the registered handler,
// and delegates retry/failure recording to the job repository.
type Executor struct {
	jobs     job.Repository
	registry *Registry
	log      *logger.Logger
}

// NewExecutor creates a new Executor.
func NewExecutor(jobs job.Repository, registry *Registry, log *logger.Logger) *Executor {
	return &Executor{jobs: jobs, registry: registry, log: log.WithField("component", "executor")}
}

// Execute runs the job and transitions it to succeeded, failed, or dead.
func (e *Executor) Execute(ctx context.Context, j *job.Job) {
	start := time.Now().UTC()

	// Apply per-job hard timeout.
	timeout := time.Duration(j.TimeoutSec) * time.Second
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Look up the handler function.
	handler, err := e.registry.Get(j.Type)
	if err != nil {
		e.log.Warn("unregistered job type",
			logger.String("job_id", j.ID.String()),
			logger.String("type", j.Type),
		)
		// Unknown job type — move directly to dead (not a transient error).
		_ = e.jobs.MarkDead(ctx, j.ID, fmt.Sprintf("no handler for type: %s", j.Type), time.Now().UTC())
		return
	}

	e.log.Info("executing job",
		logger.String("job_id", j.ID.String()),
		logger.String("type", j.Type),
		logger.Int("attempt", j.AttemptCount+1),
		logger.Int("max_retries", j.MaxRetries),
	)

	// Execute the handler.
	execErr := handler(jobCtx, j)
	duration := time.Since(start)

	if execErr == nil {
		// ── Success path ────────────────────────────────────────────────────
		if err := e.jobs.MarkSucceeded(ctx, j.ID, time.Now().UTC()); err != nil {
			e.log.Error("failed to mark job succeeded", logger.Err(err))
		}
		e.log.Info("job succeeded",
			logger.String("job_id", j.ID.String()),
			logger.Duration("duration_ms", duration.Milliseconds()),
		)
		return
	}

	// ── Failure path ─────────────────────────────────────────────────────────
	errMsg := execErr.Error()
	nextAttempt := j.AttemptCount + 1

	e.log.Warn("job failed",
		logger.String("job_id", j.ID.String()),
		logger.String("error", errMsg),
		logger.Int("attempt", nextAttempt),
		logger.Int("max_retries", j.MaxRetries),
	)

	if nextAttempt < j.MaxRetries {
		// Schedule a retry with the configured backoff.
		policy := retrysvc.Policy{
			MaxRetries: j.MaxRetries,
			Strategy:   retrysvc.Strategy(j.RetryStrategy),
			BaseDelay:  time.Duration(j.RetryDelaySec) * time.Second,
			MaxDelay:   24 * time.Hour,
			Jitter:     j.RetryStrategy == "exponential",
		}
		nextRunAt := retrysvc.NextRunAt(policy, nextAttempt, time.Now().UTC())
		if err := e.jobs.ScheduleRetry(ctx, j.ID, nextRunAt, nextAttempt); err != nil {
			e.log.Error("failed to schedule retry", logger.Err(err))
		} else {
			e.log.Info("job retry scheduled",
				logger.String("job_id", j.ID.String()),
				logger.String("next_run_at", nextRunAt.Format(time.RFC3339)),
			)
		}
	} else {
		// Max retries exhausted — move to dead.
		if err := e.jobs.MarkDead(ctx, j.ID, errMsg, time.Now().UTC()); err != nil {
			e.log.Error("failed to mark job dead", logger.Err(err))
		}
		e.log.Warn("job moved to dead letter queue",
			logger.String("job_id", j.ID.String()),
			logger.Int("attempts", nextAttempt),
		)
	}
}
