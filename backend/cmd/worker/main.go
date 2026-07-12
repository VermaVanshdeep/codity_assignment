// Command worker is the Worker Service entrypoint for the Job Scheduler.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/job-scheduler/internal/config"
	"github.com/your-org/job-scheduler/internal/domain/job"
	platformcache "github.com/your-org/job-scheduler/internal/platform/cache"
	platformdb "github.com/your-org/job-scheduler/internal/platform/db"
	"github.com/your-org/job-scheduler/internal/platform/logger"
	"github.com/your-org/job-scheduler/internal/repository/postgres"
	workerdomain "github.com/your-org/job-scheduler/internal/domain/worker"
	"github.com/your-org/job-scheduler/internal/worker"
	"github.com/your-org/job-scheduler/pkg/lock"
)

func main() {
	// ── 1. Configuration ─────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: load config: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Logger ────────────────────────────────────────────────────────────
	log := logger.New(logger.Config{
		Level:  cfg.Logger.Level,
		Format: cfg.Logger.Format,
	})
	log.Info("starting job-scheduler worker",
		logger.String("env", cfg.App.Env),
		logger.Int("concurrency", cfg.Worker.Concurrency),
	)

	// ── 3. Startup context ───────────────────────────────────────────────────
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer startupCancel()

	// ── 4. PostgreSQL ─────────────────────────────────────────────────────────
	pool, err := platformdb.Connect(startupCtx, cfg.Postgres, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", logger.Err(err))
	}
	defer pool.Close()

	// ── 5. Redis ──────────────────────────────────────────────────────────────
	redisClient, err := platformcache.New(startupCtx, cfg.Redis, log)
	if err != nil {
		log.Fatal("failed to connect to redis", logger.Err(err))
	}
	defer redisClient.Close()

	// ── 6. Repositories ───────────────────────────────────────────────────────
	queueRepo := postgres.NewQueueRepo(pool)
	jobRepo := postgres.NewJobRepo(pool)
	cronRepo := postgres.NewCronRepo(pool)
	workerRepo := postgres.NewWorkerRepo(pool)
	lockManager := lock.NewManager(redisClient)

	// ── 7. Worker Runtime ─────────────────────────────────────────────────────
	svc := worker.NewService(cfg.Worker, queueRepo, jobRepo, cronRepo, workerRepo, lockManager, log)
	
	// Register worker in DB
	hostname, _ := os.Hostname()
	wInfo := &workerdomain.Worker{
		ID:          svc.ID(), // We need to expose this
		Hostname:    hostname,
		PID:         os.Getpid(),
		Version:     "v1.0.0",
		Status:      workerdomain.StatusActive,
		Concurrency: cfg.Worker.Concurrency,
		Queues:      []string{"default", "high-priority"},
	}
	if err := workerRepo.Register(startupCtx, wInfo); err != nil {
		log.Error("failed to register worker", logger.Err(err))
	}


	// Register built-in handlers.
	registry := svc.Registry()
	registry.Register("demo.echo", func(ctx context.Context, j *job.Job) error {
		msg, ok := j.Payload["message"].(string)
		if !ok {
			return fmt.Errorf("missing or invalid 'message' in payload")
		}
		log.Info("executed demo.echo", logger.String("message", msg))
		return nil
	})

	registry.Register("demo.sleep", func(ctx context.Context, j *job.Job) error {
		durationSec, ok := j.Payload["seconds"].(float64)
		if !ok {
			return fmt.Errorf("missing or invalid 'seconds' in payload")
		}
		log.Info("executing demo.sleep", logger.Float64("seconds", durationSec))

		select {
		case <-time.After(time.Duration(durationSec) * time.Second):
			log.Info("woke up from demo.sleep")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// ── 8. Start & Graceful shutdown ──────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Info("shutdown signal received")
		cancel()
	}()

	// Start blocks until ctx is canceled, then initiates graceful shutdown.
	svc.Start(ctx)
}
