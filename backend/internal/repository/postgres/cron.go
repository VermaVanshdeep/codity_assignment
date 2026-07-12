package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-org/job-scheduler/internal/domain/cron"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// CronRepo implements cron.Repository using PostgreSQL.
type CronRepo struct {
	db *pgxpool.Pool
}

// NewCronRepo creates a new CronRepo.
func NewCronRepo(db *pgxpool.Pool) *CronRepo {
	return &CronRepo{db: db}
}

// Create inserts a new cron job definition.
func (r *CronRepo) Create(ctx context.Context, c *cron.CronJob) error {
	payload, _ := json.Marshal(c.Payload)
	const sql = `
		INSERT INTO cron_jobs (id, queue_id, name, description, cron_expr, timezone, job_type, payload, max_retries, retry_strategy, is_active, next_fire_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`

	_, err := r.db.Exec(ctx, sql,
		c.ID, c.QueueID, c.Name, c.Description, c.CronExpr, c.Timezone,
		c.JobType, payload, c.MaxRetries, c.RetryStrategy,
		c.IsActive, c.NextFireAt, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.AlreadyExists("cron job name")
		}
		return fmt.Errorf("create cron job: %w", err)
	}
	return nil
}

// GetByID retrieves a cron job by ID.
func (r *CronRepo) GetByID(ctx context.Context, id uuid.UUID) (*cron.CronJob, error) {
	const sql = `SELECT ` + cronColumns + ` FROM cron_jobs WHERE id = $1`
	c, err := scanCron(r.db.QueryRow(ctx, sql, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("cron job")
		}
		return nil, fmt.Errorf("get cron by id: %w", err)
	}
	return c, nil
}

// ListByQueueID returns all cron definitions for a queue.
func (r *CronRepo) ListByQueueID(ctx context.Context, queueID uuid.UUID) ([]*cron.CronJob, error) {
	const sql = `SELECT ` + cronColumns + ` FROM cron_jobs WHERE queue_id = $1 ORDER BY name`
	rows, err := r.db.Query(ctx, sql, queueID)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs: %w", err)
	}
	defer rows.Close()

	var crons []*cron.CronJob
	for rows.Next() {
		c, err := scanCron(rows)
		if err != nil {
			return nil, err
		}
		crons = append(crons, c)
	}
	return crons, rows.Err()
}

// ListDue returns active cron jobs whose next_fire_at <= now.
func (r *CronRepo) ListDue(ctx context.Context, now time.Time) ([]*cron.CronJob, error) {
	const sql = `
		SELECT ` + cronColumns + `
		FROM cron_jobs
		WHERE is_active = true AND next_fire_at <= $1
		ORDER BY next_fire_at`
	rows, err := r.db.Query(ctx, sql, now)
	if err != nil {
		return nil, fmt.Errorf("list due cron jobs: %w", err)
	}
	defer rows.Close()

	var crons []*cron.CronJob
	for rows.Next() {
		c, err := scanCron(rows)
		if err != nil {
			return nil, err
		}
		crons = append(crons, c)
	}
	return crons, rows.Err()
}

// Update persists changes to a cron job definition.
func (r *CronRepo) Update(ctx context.Context, c *cron.CronJob) error {
	payload, _ := json.Marshal(c.Payload)
	const sql = `
		UPDATE cron_jobs SET
			name=$2, description=$3, cron_expr=$4, timezone=$5,
			job_type=$6, payload=$7, max_retries=$8, retry_strategy=$9,
			is_active=$10, next_fire_at=$11, updated_at=$12
		WHERE id=$1`

	tag, err := r.db.Exec(ctx, sql,
		c.ID, c.Name, c.Description, c.CronExpr, c.Timezone,
		c.JobType, payload, c.MaxRetries, c.RetryStrategy,
		c.IsActive, c.NextFireAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update cron job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("cron job")
	}
	return nil
}

// UpdateFireTimes updates last_fired_at and next_fire_at after a successful fire.
func (r *CronRepo) UpdateFireTimes(ctx context.Context, id uuid.UUID, lastFiredAt, nextFireAt time.Time) error {
	const sql = `UPDATE cron_jobs SET last_fired_at=$2, next_fire_at=$3, updated_at=NOW() WHERE id=$1`
	_, err := r.db.Exec(ctx, sql, id, lastFiredAt, nextFireAt)
	return err
}

// SetActive enables or disables a cron job.
func (r *CronRepo) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	const sql = `UPDATE cron_jobs SET is_active=$2, updated_at=NOW() WHERE id=$1`
	tag, err := r.db.Exec(ctx, sql, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("cron job")
	}
	return nil
}

// Delete removes a cron job definition.
func (r *CronRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM cron_jobs WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete cron job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("cron job")
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const cronColumns = `id, queue_id, name, description, cron_expr, timezone, job_type, payload, max_retries, retry_strategy, is_active, last_fired_at, next_fire_at, created_at, updated_at`

func scanCron(row scanner) (*cron.CronJob, error) {
	var c cron.CronJob
	var payloadBytes []byte
	err := row.Scan(
		&c.ID, &c.QueueID, &c.Name, &c.Description, &c.CronExpr, &c.Timezone,
		&c.JobType, &payloadBytes, &c.MaxRetries, &c.RetryStrategy,
		&c.IsActive, &c.LastFiredAt, &c.NextFireAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &c.Payload)
	}
	return &c, nil
}
