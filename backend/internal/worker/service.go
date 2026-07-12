package worker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/config"
	domaincron "github.com/your-org/job-scheduler/internal/domain/cron"
	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/domain/queue"
	workerdomain "github.com/your-org/job-scheduler/internal/domain/worker"
	"github.com/your-org/job-scheduler/internal/platform/logger"
	"github.com/your-org/job-scheduler/pkg/lock"
)

// Service is the main entry point for a worker node.
// It orchestrates the Registry, Pool, Dispatcher, Reaper, Scheduler, and CronEngine.
type Service struct {
	id          uuid.UUID
	registry    *Registry
	pool        *Pool
	dispatcher  *Dispatcher
	reaper      *Reaper
	scheduler   *Scheduler
	cronEngine  *CronEngine
	heartbeater *Heartbeater
	log         *logger.Logger
}

// NewService constructs a fully wired worker service.
func NewService(
	cfg config.WorkerConfig,
	queues queue.Repository,
	jobs job.Repository,
	crons domaincron.Repository,
	workers workerdomain.Repository,
	lockManager *lock.Manager,
	log *logger.Logger,
) *Service {
	workerID := uuid.New()
	registry := NewRegistry()

	executor := NewExecutor(jobs, registry, log)
	pool := NewPool(cfg.Concurrency, executor, log)

	dispatcher := NewDispatcher(workerID, queues, jobs, pool, cfg.PollInterval, cfg.MaxBatchSize, log)

	reaper := NewReaper(jobs, cfg.PollInterval, cfg.VisibilityTimeout, log)
	scheduler := NewScheduler(jobs, cfg.PollInterval, log)
	cronEngine := NewCronEngine(crons, jobs, lockManager, cfg.PollInterval, 55*time.Second, log)
	heartbeater := NewHeartbeater(workerID, workers, 5*time.Second, log)

	return &Service{
		id:          workerID,
		registry:    registry,
		pool:        pool,
		dispatcher:  dispatcher,
		reaper:      reaper,
		scheduler:   scheduler,
		cronEngine:  cronEngine,
		heartbeater: heartbeater,
		log:         log.WithField("worker_id", workerID.String()),
	}
}

// Registry returns the handler registry so that main() can map job types to functions.
func (s *Service) Registry() *Registry {
	return s.registry
}

// Start launches all background loops and the worker pool.
// It blocks until the provided context is canceled.
func (s *Service) Start(ctx context.Context) {
	s.log.Info("starting worker node",
		logger.Any("handlers", s.registry.Types()),
	)

	var wg sync.WaitGroup

	// Start pool.
	s.pool.Start(ctx)

	// Start sub-components.
	components := []func(context.Context){
		s.dispatcher.Start,
		s.reaper.Start,
		s.scheduler.Start,
		s.cronEngine.Start,
		s.heartbeater.Start,
	}

	for _, c := range components {
		wg.Add(1)
		go func(startFn func(context.Context)) {
			defer wg.Done()
			startFn(ctx)
		}(c)
	}

	// Block until context is canceled.
	<-ctx.Done()
	s.log.Info("worker node context canceled, initiating graceful shutdown")

	// Wait for background loops to exit.
	wg.Wait()

	// Drain the worker pool (waits for active jobs to finish).
	s.pool.Stop()

	s.log.Info("worker node stopped cleanly")
}


func (s *Service) ID() uuid.UUID {
	return s.id
}
