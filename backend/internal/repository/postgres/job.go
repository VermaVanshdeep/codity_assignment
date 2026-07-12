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
	"github.com/your-org/job-scheduler/internal/domain/job"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// JobRepo implements job.Repository using PostgreSQL.
type JobRepo struct {
	db *pgxpool.Pool
}

// NewJobRepo creates a new JobRepo.
func NewJobRepo(db *pgxpool.Pool) *JobRepo {
	return &JobRepo{db: db}
}

// Create inserts a new job.
func (r *JobRepo) Create(ctx context.Context, j *job.Job) error {
	payload, _ := json.Marshal(j.Payload)
	meta, _ := json.Marshal(j.Metadata)

	const sql = `
		INSERT INTO jobs (
			id, queue_id, batch_id, parent_job_id, cron_job_id,
			type, payload, idempotency_key,
			status, priority, run_at,
			max_retries, attempt_count, retry_strategy, retry_delay_sec,
			timeout_sec, tags, metadata, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,
			$9,$10,$11,
			$12,$13,$14,$15,
			$16,$17,$18,$19,$20
		)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING`

	_, err := r.db.Exec(ctx, sql,
		j.ID, j.QueueID, j.BatchID, j.ParentJobID, j.CronJobID,
		j.Type, payload, j.IdempotencyKey,
		j.Status, j.Priority, j.RunAt,
		j.MaxRetries, j.AttemptCount, j.RetryStrategy, j.RetryDelaySec,
		j.TimeoutSec, j.Tags, meta, j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

// GetByID retrieves a job by primary key.
func (r *JobRepo) GetByID(ctx context.Context, id uuid.UUID) (*job.Job, error) {
	const sql = `SELECT ` + jobColumns + ` FROM jobs WHERE id = $1`
	j, err := scanJob(r.db.QueryRow(ctx, sql, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("job")
		}
		return nil, fmt.Errorf("get job by id: %w", err)
	}
	return j, nil
}

// GetByIdempotencyKey retrieves a job by its idempotency key.
func (r *JobRepo) GetByIdempotencyKey(ctx context.Context, key string) (*job.Job, error) {
	const sql = `SELECT ` + jobColumns + ` FROM jobs WHERE idempotency_key = $1`
	j, err := scanJob(r.db.QueryRow(ctx, sql, key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("job")
		}
		return nil, fmt.Errorf("get job by idempotency key: %w", err)
	}
	return j, nil
}

// List returns jobs for a queue with filtering and pagination.
func (r *JobRepo) List(ctx context.Context, queueID uuid.UUID, filter job.ListFilter) ([]*job.Job, int64, error) {
	// Build dynamic where clause.
	args := []any{queueID}
	where := "queue_id = $1"
	idx := 2

	if filter.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, string(*filter.Status))
		idx++
	}
	if filter.Type != nil {
		where += fmt.Sprintf(" AND type = $%d", idx)
		args = append(args, *filter.Type)
		idx++
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filter.Offset

	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM jobs WHERE %s`, where)
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count jobs: %w", err)
	}

	args = append(args, limit, offset)
	dataSQL := fmt.Sprintf(`SELECT `+jobColumns+` FROM jobs WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, idx, idx+1)
	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, j)
	}
	return jobs, total, rows.Err()
}

// ClaimBatch is the heart of the dispatcher.
// It atomically claims up to batchSize eligible jobs using SELECT … FOR UPDATE SKIP LOCKED.
// This is deadlock-free: SKIP LOCKED silently skips rows already locked by other workers.
// Concurrency enforcement: it first checks how many jobs are already running for the queue.
func (r *JobRepo) ClaimBatch(ctx context.Context, queueID, workerID uuid.UUID, batchSize, concurrencyLimit int) ([]*job.Job, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Count currently running jobs for this queue.
	var runningCount int
	const countSQL = `SELECT COUNT(*) FROM jobs WHERE queue_id = $1 AND status = 'running'`
	if err := tx.QueryRow(ctx, countSQL, queueID).Scan(&runningCount); err != nil {
		return nil, fmt.Errorf("count running jobs: %w", err)
	}

	available := concurrencyLimit - runningCount
	if available <= 0 {
		_ = tx.Rollback(ctx)
		return nil, nil // queue at concurrency limit — no error, just no jobs
	}
	if available < batchSize {
		batchSize = available
	}

	// Claim jobs atomically.
	const claimSQL = `
		SELECT ` + jobColumns + `
		FROM jobs
		WHERE queue_id = $1
		  AND status IN ('pending')
		  AND run_at <= NOW()
		ORDER BY priority DESC, run_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`

	rows, err := tx.Query(ctx, claimSQL, queueID, batchSize)
	if err != nil {
		return nil, fmt.Errorf("claim query: %w", err)
	}

	var claimed []*job.Job
	var claimedIDs []uuid.UUID
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		claimed = append(claimed, j)
		claimedIDs = append(claimedIDs, j.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(claimedIDs) == 0 {
		_ = tx.Rollback(ctx)
		return nil, nil
	}

	// Mark all claimed jobs as running.
	const updateSQL = `
		UPDATE jobs SET
			status     = 'running',
			claimed_by = $1,
			claimed_at = NOW(),
			started_at = NOW(),
			updated_at = NOW()
		WHERE id = ANY($2)`

	if _, err := tx.Exec(ctx, updateSQL, workerID, claimedIDs); err != nil {
		return nil, fmt.Errorf("update claimed jobs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim: %w", err)
	}

	return claimed, nil
}

// UpdateStatus sets a job's status directly.
func (r *JobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status job.Status) error {
	const sql = `UPDATE jobs SET status=$2, updated_at=NOW() WHERE id=$1`
	_, err := r.db.Exec(ctx, sql, id, string(status))
	return err
}

// MarkSucceeded transitions a job to succeeded and records completion time.
func (r *JobRepo) MarkSucceeded(ctx context.Context, id uuid.UUID, completedAt time.Time) error {
	const sql = `
		UPDATE jobs SET
			status=$2, completed_at=$3, updated_at=NOW()
		WHERE id=$1`
	_, err := r.db.Exec(ctx, sql, id, string(job.StatusSucceeded), completedAt)
	return err
}

// MarkFailed transitions a running job to failed and increments attempt_count.
func (r *JobRepo) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, failedAt time.Time) error {
	const sql = `
		UPDATE jobs SET
			status=$2, attempt_count=attempt_count+1,
			last_error=$3, last_error_at=$4, updated_at=NOW()
		WHERE id=$1`
	_, err := r.db.Exec(ctx, sql, id, string(job.StatusFailed), errMsg, failedAt)
	return err
}

// MarkDead moves a job to the dead state (max retries exhausted).
func (r *JobRepo) MarkDead(ctx context.Context, id uuid.UUID, errMsg string, failedAt time.Time) error {
	const sql = `
		UPDATE jobs SET
			status=$2, attempt_count=attempt_count+1,
			last_error=$3, last_error_at=$4,
			completed_at=$4, updated_at=NOW()
		WHERE id=$1`
	_, err := r.db.Exec(ctx, sql, id, string(job.StatusDead), errMsg, failedAt)
	return err
}

// ScheduleRetry resets a failed job for its next retry attempt.
func (r *JobRepo) ScheduleRetry(ctx context.Context, id uuid.UUID, runAt time.Time, attemptCount int) error {
	const sql = `
		UPDATE jobs SET
			status='pending', run_at=$2, attempt_count=$3,
			claimed_by=NULL, claimed_at=NULL, updated_at=NOW()
		WHERE id=$1`
	_, err := r.db.Exec(ctx, sql, id, runAt, attemptCount)
	return err
}

// Cancel transitions a job to cancelled (only possible from pending/scheduled).
func (r *JobRepo) Cancel(ctx context.Context, id uuid.UUID) error {
	const sql = `
		UPDATE jobs SET status='cancelled', updated_at=NOW()
		WHERE id=$1 AND status IN ('pending','scheduled')`
	tag, err := r.db.Exec(ctx, sql, id)
	if err != nil {
		return fmt.Errorf("cancel job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.Conflict("job cannot be cancelled in its current state")
	}
	return nil
}

// PromoteScheduled moves scheduled jobs whose run_at <= NOW() to pending.
// Called by the scheduler loop every tick.
func (r *JobRepo) PromoteScheduled(ctx context.Context) (int64, error) {
	const sql = `
		UPDATE jobs SET status='pending', updated_at=NOW()
		WHERE status='scheduled' AND run_at <= NOW()`
	tag, err := r.db.Exec(ctx, sql)
	if err != nil {
		return 0, fmt.Errorf("promote scheduled: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ReclaimStale finds running jobs whose claimed_at has exceeded the visibility timeout
// (indicating the worker died) and resets them to pending for retry.
func (r *JobRepo) ReclaimStale(ctx context.Context, visibilityTimeout time.Duration) (int64, error) {
	const sql = `
		UPDATE jobs SET
			status='pending', claimed_by=NULL, claimed_at=NULL, updated_at=NOW()
		WHERE status='running'
		  AND claimed_at < NOW() - ($1 || ' seconds')::INTERVAL`
	secs := int(visibilityTimeout.Seconds())
	tag, err := r.db.Exec(ctx, sql, fmt.Sprintf("%d", secs))
	if err != nil {
		return 0, fmt.Errorf("reclaim stale: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const jobColumns = `
	id, queue_id, batch_id, parent_job_id, cron_job_id,
	type, payload, idempotency_key,
	status, priority, run_at,
	max_retries, attempt_count, retry_strategy, retry_delay_sec,
	last_error, last_error_at,
	timeout_sec, claimed_by, claimed_at, started_at, completed_at,
	tags, metadata, created_at, updated_at`

func scanJob(row scanner) (*job.Job, error) {
	var j job.Job
	var status string
	var payloadBytes, metaBytes []byte

	err := row.Scan(
		&j.ID, &j.QueueID, &j.BatchID, &j.ParentJobID, &j.CronJobID,
		&j.Type, &payloadBytes, &j.IdempotencyKey,
		&status, &j.Priority, &j.RunAt,
		&j.MaxRetries, &j.AttemptCount, &j.RetryStrategy, &j.RetryDelaySec,
		&j.LastError, &j.LastErrorAt,
		&j.TimeoutSec, &j.ClaimedBy, &j.ClaimedAt, &j.StartedAt, &j.CompletedAt,
		&j.Tags, &metaBytes, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	j.Status = job.Status(status)
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &j.Payload)
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &j.Metadata)
	}
	return &j, nil
}
