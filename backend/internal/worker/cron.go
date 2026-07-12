package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	domaincron "github.com/your-org/job-scheduler/internal/domain/cron"
	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/platform/logger"
	"github.com/your-org/job-scheduler/pkg/lock"
)

// CronEngine runs a background loop to evaluate and fire cron jobs.
type CronEngine struct {
	crons        domaincron.Repository
	jobs         job.Repository
	lockManager  *lock.Manager
	pollInterval time.Duration
	lockTTL      time.Duration
	parser       cron.Parser
	log          *logger.Logger
}

// NewCronEngine creates a new CronEngine.
func NewCronEngine(
	crons domaincron.Repository,
	jobs job.Repository,
	lockManager *lock.Manager,
	pollInterval time.Duration,
	lockTTL time.Duration,
	log *logger.Logger,
) *CronEngine {
	// Standard parser: Minute, Hour, Dom, Month, Dow
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	return &CronEngine{
		crons:        crons,
		jobs:         jobs,
		lockManager:  lockManager,
		pollInterval: pollInterval,
		lockTTL:      lockTTL,
		parser:       parser,
		log:          log.WithField("component", "cron_engine"),
	}
}

// Start runs the cron evaluation loop.
func (e *CronEngine) Start(ctx context.Context) {
	e.log.Info("cron engine started", logger.String("poll_interval", e.pollInterval.String()))
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.log.Info("cron engine stopping")
			return
		case <-ticker.C:
			e.evaluate(ctx)
		}
	}
}

func (e *CronEngine) evaluate(ctx context.Context) {
	now := time.Now().UTC()

	// 1. Fetch all due cron jobs.
	dueCrons, err := e.crons.ListDue(ctx, now)
	if err != nil {
		e.log.Error("failed to list due cron jobs", logger.Err(err))
		return
	}

	for _, c := range dueCrons {
		e.fireCron(ctx, c, now)
	}
}

func (e *CronEngine) fireCron(ctx context.Context, c *domaincron.CronJob, now time.Time) {
	// 2. Try to acquire a distributed lock for this specific cron job.
	// The lock key acts as a deduplication mechanism across multiple scheduler instances.
	lockName := fmt.Sprintf("cron_fire:%s:%d", c.ID.String(), now.Unix()/60) // lock per minute tick

	err := e.lockManager.TryWithLock(ctx, lockName, e.lockTTL, func(ctx context.Context) error {
		return e.executeFire(ctx, c, now)
	})

	if err != nil && err != lock.ErrNotAcquired {
		e.log.Error("error evaluating cron lock", logger.String("cron_id", c.ID.String()), logger.Err(err))
	}
}

func (e *CronEngine) executeFire(ctx context.Context, c *domaincron.CronJob, now time.Time) error {
	// Compute the *next* fire time.
	schedule, err := e.parser.Parse(c.CronExpr)
	if err != nil {
		e.log.Error("invalid cron expression", logger.String("cron_id", c.ID.String()), logger.String("expr", c.CronExpr))
		// Disable it so we don't keep polling a broken expression
		_ = e.crons.SetActive(ctx, c.ID, false)
		return err
	}

	nextFireAt := schedule.Next(now)

	// Build the spawned job.
	j := &job.Job{
		ID:           uuid.New(),
		QueueID:      c.QueueID,
		CronJobID:    &c.ID,
		Type:         c.JobType,
		Payload:      c.Payload,
		Status:       job.StatusPending,
		Priority:     5, // default
		RunAt:        now,
		AttemptCount: 0,
		Metadata:     make(map[string]any),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if c.MaxRetries != nil {
		j.MaxRetries = *c.MaxRetries
	}
	if c.RetryStrategy != nil {
		j.RetryStrategy = *c.RetryStrategy
	}

	j.Metadata["spawned_by"] = "cron"
	j.Metadata["cron_name"] = c.Name

	// Transactionally insert job and update cron timestamps.
	// For simplicity in this iteration, we do them sequentially. In production, this should be a DB tx.
	if err := e.jobs.Create(ctx, j); err != nil {
		return fmt.Errorf("spawn job for cron: %w", err)
	}

	if err := e.crons.UpdateFireTimes(ctx, c.ID, now, nextFireAt); err != nil {
		e.log.Error("failed to update cron timestamps", logger.String("cron_id", c.ID.String()), logger.Err(err))
		// Job was created but cron wasn't updated. It will fire again next tick unless lock holds.
	}

	e.log.Info("cron job fired",
		logger.String("cron_id", c.ID.String()),
		logger.String("job_id", j.ID.String()),
		logger.String("next_fire_at", nextFireAt.Format(time.RFC3339)),
	)

	return nil
}
