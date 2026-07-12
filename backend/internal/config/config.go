// Package config centralizes all application configuration loaded from environment
// variables. Using Viper allows reading from .env files in development and from
// real environment variables in production (Docker, Kubernetes, etc.).
//
// Design: A single Config struct is populated at startup and passed via dependency
// injection. No service reads env vars directly — they all receive Config.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration for the entire application.
type Config struct {
	App       AppConfig
	JWT       JWTConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Worker    WorkerConfig
	Scheduler SchedulerConfig
	Metrics   MetricsConfig
	Logger    LoggerConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

// AppConfig holds general application settings.
type AppConfig struct {
	Env     string // development | staging | production
	Port    int
	BaseURL string
}

// JWTConfig holds JWT token configuration.
type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// PostgresConfig holds database connection parameters.
type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN returns the PostgreSQL data source name for pgx.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// URL returns a PostgreSQL connection URL (used by golang-migrate).
func (c PostgresConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// RedisConfig holds Redis connection parameters.
type RedisConfig struct {
	Host       string
	Port       int
	Password   string
	DB         int
	MaxRetries int
	PoolSize   int
}

// Addr returns the Redis address string.
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// WorkerConfig holds worker runtime parameters.
type WorkerConfig struct {
	Concurrency       int
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	VisibilityTimeout time.Duration
	DrainTimeout      time.Duration
	MaxBatchSize      int
}

// SchedulerConfig holds scheduler loop parameters.
type SchedulerConfig struct {
	TickInterval time.Duration
	CronLockTTL  time.Duration
}

// MetricsConfig holds metrics flush parameters.
type MetricsConfig struct {
	FlushInterval time.Duration
}

// LoggerConfig holds logging settings.
type LoggerConfig struct {
	Level  string // debug | info | warn | error
	Format string // json | pretty
}

// CORSConfig holds allowed CORS origins.
type CORSConfig struct {
	AllowedOrigins []string
}

// RateLimitConfig holds rate limiting parameters.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
}

// Load reads configuration from environment variables (and optionally a .env file).
// It sets sensible defaults for every field so the app starts cleanly in dev.
func Load() (*Config, error) {
	v := viper.New()

	// Allow reading from a .env file when present (dev convenience).
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	// Don't fail if .env is missing — in production, real env vars are used.
	_ = v.ReadInConfig()

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults(v)

	cfg := &Config{
		App: AppConfig{
			Env:     v.GetString("APP_ENV"),
			Port:    v.GetInt("APP_PORT"),
			BaseURL: v.GetString("APP_BASE_URL"),
		},
		JWT: JWTConfig{
			Secret:          v.GetString("JWT_SECRET"),
			AccessTokenTTL:  v.GetDuration("JWT_ACCESS_TOKEN_TTL"),
			RefreshTokenTTL: v.GetDuration("JWT_REFRESH_TOKEN_TTL"),
		},
		Postgres: PostgresConfig{
			Host:            v.GetString("POSTGRES_HOST"),
			Port:            v.GetInt("POSTGRES_PORT"),
			User:            v.GetString("POSTGRES_USER"),
			Password:        v.GetString("POSTGRES_PASSWORD"),
			DBName:          v.GetString("POSTGRES_DB"),
			SSLMode:         v.GetString("POSTGRES_SSL_MODE"),
			MaxOpenConns:    v.GetInt("POSTGRES_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("POSTGRES_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetDuration("POSTGRES_CONN_MAX_LIFETIME"),
		},
		Redis: RedisConfig{
			Host:       v.GetString("REDIS_HOST"),
			Port:       v.GetInt("REDIS_PORT"),
			Password:   v.GetString("REDIS_PASSWORD"),
			DB:         v.GetInt("REDIS_DB"),
			MaxRetries: v.GetInt("REDIS_MAX_RETRIES"),
			PoolSize:   v.GetInt("REDIS_POOL_SIZE"),
		},
		Worker: WorkerConfig{
			Concurrency:       v.GetInt("WORKER_CONCURRENCY"),
			PollInterval:      v.GetDuration("WORKER_POLL_INTERVAL"),
			HeartbeatInterval: v.GetDuration("WORKER_HEARTBEAT_INTERVAL"),
			HeartbeatTimeout:  v.GetDuration("WORKER_HEARTBEAT_TIMEOUT"),
			VisibilityTimeout: v.GetDuration("WORKER_VISIBILITY_TIMEOUT"),
			DrainTimeout:      v.GetDuration("WORKER_DRAIN_TIMEOUT"),
			MaxBatchSize:      v.GetInt("WORKER_MAX_BATCH_SIZE"),
		},
		Scheduler: SchedulerConfig{
			TickInterval: v.GetDuration("SCHEDULER_TICK_INTERVAL"),
			CronLockTTL:  v.GetDuration("SCHEDULER_CRON_LOCK_TTL"),
		},
		Metrics: MetricsConfig{
			FlushInterval: v.GetDuration("METRICS_FLUSH_INTERVAL"),
		},
		Logger: LoggerConfig{
			Level:  v.GetString("LOG_LEVEL"),
			Format: v.GetString("LOG_FORMAT"),
		},
		CORS: CORSConfig{
			AllowedOrigins: strings.Split(v.GetString("CORS_ALLOWED_ORIGINS"), ","),
		},
		RateLimit: RateLimitConfig{
			Enabled:           v.GetBool("RATE_LIMIT_ENABLED"),
			RequestsPerMinute: v.GetInt("RATE_LIMIT_REQUESTS_PER_MIN"),
		},
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// setDefaults registers safe defaults for every config key.
func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("APP_BASE_URL", "http://localhost:8080")

	v.SetDefault("JWT_ACCESS_TOKEN_TTL", "15m")
	v.SetDefault("JWT_REFRESH_TOKEN_TTL", "168h") // 7 days

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", 5432)
	v.SetDefault("POSTGRES_SSL_MODE", "disable")
	v.SetDefault("POSTGRES_MAX_OPEN_CONNS", 25)
	v.SetDefault("POSTGRES_MAX_IDLE_CONNS", 5)
	v.SetDefault("POSTGRES_CONN_MAX_LIFETIME", "5m")

	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_MAX_RETRIES", 3)
	v.SetDefault("REDIS_POOL_SIZE", 10)

	v.SetDefault("WORKER_CONCURRENCY", 10)
	v.SetDefault("WORKER_POLL_INTERVAL", "1s")
	v.SetDefault("WORKER_HEARTBEAT_INTERVAL", "5s")
	v.SetDefault("WORKER_HEARTBEAT_TIMEOUT", "30s")
	v.SetDefault("WORKER_VISIBILITY_TIMEOUT", "5m")
	v.SetDefault("WORKER_DRAIN_TIMEOUT", "60s")
	v.SetDefault("WORKER_MAX_BATCH_SIZE", 10)

	v.SetDefault("SCHEDULER_TICK_INTERVAL", "1s")
	v.SetDefault("SCHEDULER_CRON_LOCK_TTL", "55s")

	v.SetDefault("METRICS_FLUSH_INTERVAL", "60s")

	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")

	v.SetDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	v.SetDefault("RATE_LIMIT_ENABLED", true)
	v.SetDefault("RATE_LIMIT_REQUESTS_PER_MIN", 120)
}

// validate checks that required fields are not empty.
func validate(cfg *Config) error {
	if cfg.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET must be set")
	}
	if len(cfg.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.Postgres.User == "" {
		return fmt.Errorf("POSTGRES_USER must be set")
	}
	if cfg.Postgres.DBName == "" {
		return fmt.Errorf("POSTGRES_DB must be set")
	}
	return nil
}
