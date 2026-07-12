// Command api is the HTTP API server for the Job Scheduler.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	redisStorage "github.com/gofiber/storage/redis/v3"

	"github.com/your-org/job-scheduler/internal/config"
	authhandler "github.com/your-org/job-scheduler/internal/handler/auth"
	batchhandler "github.com/your-org/job-scheduler/internal/handler/batch"
	cronhandler "github.com/your-org/job-scheduler/internal/handler/cron"
	execloghandler "github.com/your-org/job-scheduler/internal/handler/execlog"
	jobhandler "github.com/your-org/job-scheduler/internal/handler/job"
	metricshandler "github.com/your-org/job-scheduler/internal/handler/metrics"
	orghandler "github.com/your-org/job-scheduler/internal/handler/org"
	projecthandler "github.com/your-org/job-scheduler/internal/handler/project"
	queuehandler "github.com/your-org/job-scheduler/internal/handler/queue"
	wshandler "github.com/your-org/job-scheduler/internal/handler/ws"
	"github.com/your-org/job-scheduler/internal/middleware"
	platformcache "github.com/your-org/job-scheduler/internal/platform/cache"
	platformdb "github.com/your-org/job-scheduler/internal/platform/db"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
	"github.com/your-org/job-scheduler/internal/platform/logger"
	"github.com/your-org/job-scheduler/internal/repository/postgres"
	authsvc "github.com/your-org/job-scheduler/internal/service/auth"
	batchsvc "github.com/your-org/job-scheduler/internal/service/batch"
	cronsvc "github.com/your-org/job-scheduler/internal/service/cron"
	execlogsvc "github.com/your-org/job-scheduler/internal/service/execlog"
	jobsvc "github.com/your-org/job-scheduler/internal/service/job"
	metricssvc "github.com/your-org/job-scheduler/internal/service/metrics"
	orgsvc "github.com/your-org/job-scheduler/internal/service/org"
	projectsvc "github.com/your-org/job-scheduler/internal/service/project"
	queuesvc "github.com/your-org/job-scheduler/internal/service/queue"
	"github.com/your-org/job-scheduler/pkg/validator"
)

func main() {
	// ── 1. Config ─────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: load config: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Logger ─────────────────────────────────────────────────────────────
	log := logger.New(logger.Config{Level: cfg.Logger.Level, Format: cfg.Logger.Format})
	log.Info("starting job-scheduler API",
		logger.String("env", cfg.App.Env),
		logger.Int("port", cfg.App.Port),
	)

	// ── 3. Infrastructure ─────────────────────────────────────────────────────
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer startupCancel()

	pool, err := platformdb.Connect(startupCtx, cfg.Postgres, log)
	if err != nil {
		log.Fatal("postgres connection failed", logger.Err(err))
	}
	defer pool.Close()

	redisClient, err := platformcache.New(startupCtx, cfg.Redis, log)
	if err != nil {
		log.Fatal("redis connection failed", logger.Err(err))
	}
	defer redisClient.Close()

	// ── 4. Migrations ─────────────────────────────────────────────────────────
	if err := platformdb.RunMigrations(cfg.Postgres.URL(), "migrations", log); err != nil {
		log.Fatal("migration failed", logger.Err(err))
	}

	// ── 5. Repositories ───────────────────────────────────────────────────────
	userRepo := postgres.NewUserRepo(pool)
	refreshTokenRepo := postgres.NewRefreshTokenRepo(pool)
	orgRepo := postgres.NewOrgRepo(pool)
	projectRepo := postgres.NewProjectRepo(pool)
	queueRepo := postgres.NewQueueRepo(pool)
	jobRepo := postgres.NewJobRepo(pool)
	batchRepo := postgres.NewBatchRepo(pool)
	cronRepo := postgres.NewCronRepo(pool)
	metricsRepo := postgres.NewMetricsRepo(pool)
	execLogRepo := postgres.NewExecLogRepo(pool)

	// ── 6. Services ───────────────────────────────────────────────────────────
	authService := authsvc.NewService(userRepo, refreshTokenRepo, cfg.JWT, log)
	orgService := orgsvc.NewService(orgRepo, log)
	projectService := projectsvc.NewService(projectRepo, log)
	queueService := queuesvc.NewService(queueRepo, log)
	jobService := jobsvc.NewService(jobRepo, queueRepo, log)
	batchService := batchsvc.NewService(batchRepo, jobRepo, log)
	cronService := cronsvc.NewService(cronRepo, log)
	metricsService := metricssvc.NewService(metricsRepo, log)
	execLogService := execlogsvc.NewService(execLogRepo, log)

	// ── 7. Shared ─────────────────────────────────────────────────────────────
	validate := validator.New()

	// ── 8. Fiber ──────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:               "Job Scheduler API v1",
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           120 * time.Second,
		ErrorHandler:          buildErrorHandler(log),
		DisableStartupMessage: cfg.App.Env == "production",
	})

	app.Use(recover.New(recover.Config{EnableStackTrace: cfg.App.Env != "production"}))
	app.Use(requestid.New())
	app.Use(helmet.New())
	app.Use(compress.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     joinStrings(cfg.CORS.AllowedOrigins),
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Request-ID",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// Rate Limiter
	store := redisStorage.New(redisStorage.Config{
		Host:  cfg.Redis.Host,
		Port:  cfg.Redis.Port,
		Reset: false,
	})
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    apperrors.CodeRateLimited,
					"message": "rate limit exceeded",
				},
			})
		},
		Storage: store,
	}))

	// ── 9. Routes ─────────────────────────────────────────────────────────────
	app.Get("/health", healthHandler(pool))

	api := app.Group("/api/v1")

	// Auth — public.
	authhandler.New(authService, validate).RegisterRoutes(api.Group("/auth"))

	// Orgs — all require JWT.
	orgGroup := api.Group("/orgs", middleware.AuthRequired(authService))
	orghandler.New(orgService, validate).RegisterRoutes(orgGroup, orgRepo)

	// Projects — nested under orgs, all require JWT.
	projectGroup := api.Group("/orgs/:orgId/projects", middleware.AuthRequired(authService))
	projecthandler.New(projectService, validate).RegisterRoutes(projectGroup, orgRepo, projectRepo)

	// Queues — nested under projects.
	queueGroup := api.Group("/projects/:projectId/queues", middleware.AuthRequired(authService))
	queuehandler.New(queueService, validate).RegisterRoutes(queueGroup, projectRepo)

	// Jobs — nested under queues.
	jobGroup := api.Group("/projects/:projectId/queues/:queueId/jobs", middleware.AuthRequired(authService))
	jobhandler.New(jobService, validate).RegisterRoutes(jobGroup, projectRepo)

	// Batches & Cron — nested under queues.
	batchGroup := api.Group("/batches", middleware.AuthRequired(authService))
	batchhandler.New(batchService, validate).RegisterRoutes(queueGroup, batchGroup)

	cronGroup := api.Group("/cron", middleware.AuthRequired(authService))
	cronhandler.New(cronService, validate).RegisterRoutes(queueGroup, cronGroup)

	// Metrics
	metricsGroup := api.Group("/metrics", middleware.AuthRequired(authService))
	metricshandler.New(metricsService).RegisterRoutes(metricsGroup)

	// Execution Logs
	globalJobGroup := api.Group("/jobs", middleware.AuthRequired(authService))
	execloghandler.New(execLogService).RegisterRoutes(globalJobGroup)

	// WebSocket Hub
	wsHub := wshandler.NewHub(redisClient, log)
	wsGroup := api.Group("/ws", middleware.AuthRequired(authService))
	wsHub.RegisterRoutes(wsGroup)

	// ── 10. Start ─────────────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%d", cfg.App.Port)
		log.Info("HTTP server listening", logger.String("addr", addr))
		if listenErr := app.Listen(addr); listenErr != nil {
			log.Fatal("server error", logger.Err(listenErr))
		}
	}()

	<-quit
	log.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if shutdownErr := app.ShutdownWithContext(shutdownCtx); shutdownErr != nil {
		log.Error("graceful shutdown error", logger.Err(shutdownErr))
	}
	log.Info("API server stopped cleanly")
}

func healthHandler(pool *platformdb.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if pingErr := pool.Ping(c.Context()); pingErr != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "degraded",
				"db":     "unreachable",
			})
		}
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "job-scheduler-api",
			"time":    time.Now().UTC(),
		})
	}
}

func buildErrorHandler(log *logger.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		message := "an unexpected error occurred"

		if fiberErr, ok := err.(*fiber.Error); ok {
			code = fiberErr.Code
			message = fiberErr.Message
		}

		if code == fiber.StatusInternalServerError {
			log.Error("unhandled error",
				logger.String("path", c.Path()),
				logger.String("method", c.Method()),
				logger.Err(err),
			)
		}

		return c.Status(code).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    apperrors.CodeInternal,
				"message": message,
			},
		})
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
