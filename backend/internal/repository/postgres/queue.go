package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/your-org/job-scheduler/internal/domain/queue"
	apperrors "github.com/your-org/job-scheduler/internal/platform/errors"
)

// QueueRepo implements queue.Repository using PostgreSQL.
type QueueRepo struct {
	db *pgxpool.Pool
}

// NewQueueRepo creates a new QueueRepo.
func NewQueueRepo(db *pgxpool.Pool) *QueueRepo {
	return &QueueRepo{db: db}
}

// Create inserts a new queue.
func (r *QueueRepo) Create(ctx context.Context, q *queue.Queue) error {
	meta, _ := json.Marshal(q.Metadata)
	const sql = `
		INSERT INTO queues (
			id, project_id, name, description, priority, concurrency,
			max_retries, retry_strategy, retry_delay_sec,
			visibility_timeout_sec, job_timeout_sec,
			is_paused, is_dlq, dlq_queue_id, metadata, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`

	_, err := r.db.Exec(ctx, sql,
		q.ID, q.ProjectID, q.Name, q.Description, q.Priority, q.Concurrency,
		q.MaxRetries, q.RetryStrategy, q.RetryDelaySec,
		q.VisibilityTimeoutSec, q.JobTimeoutSec,
		q.IsPaused, q.IsDLQ, q.DLQQueueID, meta, q.CreatedAt, q.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.AlreadyExists("queue name")
		}
		return fmt.Errorf("create queue: %w", err)
	}
	return nil
}

// GetByID retrieves a queue by primary key.
func (r *QueueRepo) GetByID(ctx context.Context, id uuid.UUID) (*queue.Queue, error) {
	const sql = `SELECT ` + queueColumns + ` FROM queues WHERE id = $1`
	q, err := scanQueue(r.db.QueryRow(ctx, sql, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("queue")
		}
		return nil, fmt.Errorf("get queue by id: %w", err)
	}
	return q, nil
}

// GetByName retrieves a queue by project + name.
func (r *QueueRepo) GetByName(ctx context.Context, projectID uuid.UUID, name string) (*queue.Queue, error) {
	const sql = `SELECT ` + queueColumns + ` FROM queues WHERE project_id = $1 AND name = $2`
	q, err := scanQueue(r.db.QueryRow(ctx, sql, projectID, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("queue")
		}
		return nil, fmt.Errorf("get queue by name: %w", err)
	}
	return q, nil
}

// ListByProjectID returns all queues for a project.
func (r *QueueRepo) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]*queue.Queue, error) {
	const sql = `SELECT ` + queueColumns + ` FROM queues WHERE project_id = $1 ORDER BY priority, name`
	rows, err := r.db.Query(ctx, sql, projectID)
	if err != nil {
		return nil, fmt.Errorf("list queues: %w", err)
	}
	defer rows.Close()
	return scanQueues(rows)
}

// ListActivePrioritized returns non-paused queues ordered by priority ascending.
func (r *QueueRepo) ListActivePrioritized(ctx context.Context) ([]*queue.Queue, error) {
	const sql = `SELECT ` + queueColumns + ` FROM queues WHERE is_paused = false ORDER BY priority ASC, name`
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("list active queues: %w", err)
	}
	defer rows.Close()
	return scanQueues(rows)
}

// Update persists changes to a queue.
func (r *QueueRepo) Update(ctx context.Context, q *queue.Queue) error {
	meta, _ := json.Marshal(q.Metadata)
	const sql = `
		UPDATE queues SET
			name=$2, description=$3, priority=$4, concurrency=$5,
			max_retries=$6, retry_strategy=$7, retry_delay_sec=$8,
			visibility_timeout_sec=$9, job_timeout_sec=$10,
			dlq_queue_id=$11, metadata=$12, updated_at=$13
		WHERE id=$1`

	tag, err := r.db.Exec(ctx, sql,
		q.ID, q.Name, q.Description, q.Priority, q.Concurrency,
		q.MaxRetries, q.RetryStrategy, q.RetryDelaySec,
		q.VisibilityTimeoutSec, q.JobTimeoutSec,
		q.DLQQueueID, meta, q.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update queue: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("queue")
	}
	return nil
}

// Delete removes a queue.
func (r *QueueRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM queues WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("queue")
	}
	return nil
}

// SetPaused updates only the is_paused flag — a lightweight operation.
func (r *QueueRepo) SetPaused(ctx context.Context, id uuid.UUID, paused bool) error {
	const sql = `UPDATE queues SET is_paused=$2, updated_at=NOW() WHERE id=$1`
	tag, err := r.db.Exec(ctx, sql, id, paused)
	if err != nil {
		return fmt.Errorf("set queue paused: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("queue")
	}
	return nil
}

// GetStats returns live job counts for a queue via a single aggregated query.
func (r *QueueRepo) GetStats(ctx context.Context, id uuid.UUID) (*queue.Stats, error) {
	const sql = `
		SELECT
			$1::uuid,
			COUNT(*) FILTER (WHERE status = 'pending')   AS pending,
			COUNT(*) FILTER (WHERE status = 'scheduled') AS scheduled,
			COUNT(*) FILTER (WHERE status = 'running')   AS running,
			COUNT(*) FILTER (WHERE status = 'succeeded') AS succeeded,
			COUNT(*) FILTER (WHERE status = 'failed')    AS failed,
			COUNT(*) FILTER (WHERE status = 'dead')      AS dead,
			COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled
		FROM jobs WHERE queue_id = $1`

	var s queue.Stats
	err := r.db.QueryRow(ctx, sql, id).Scan(
		&s.QueueID,
		&s.PendingCount, &s.ScheduledCount, &s.RunningCount,
		&s.SucceededCount, &s.FailedCount, &s.DeadCount, &s.CancelledCount,
	)
	if err != nil {
		return nil, fmt.Errorf("get queue stats: %w", err)
	}
	return &s, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

const queueColumns = `
	id, project_id, name, description, priority, concurrency,
	max_retries, retry_strategy, retry_delay_sec,
	visibility_timeout_sec, job_timeout_sec,
	is_paused, is_dlq, dlq_queue_id, metadata, created_at, updated_at`

func scanQueue(row scanner) (*queue.Queue, error) {
	var q queue.Queue
	var metaBytes []byte
	err := row.Scan(
		&q.ID, &q.ProjectID, &q.Name, &q.Description, &q.Priority, &q.Concurrency,
		&q.MaxRetries, &q.RetryStrategy, &q.RetryDelaySec,
		&q.VisibilityTimeoutSec, &q.JobTimeoutSec,
		&q.IsPaused, &q.IsDLQ, &q.DLQQueueID, &metaBytes, &q.CreatedAt, &q.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &q.Metadata)
	}
	return &q, nil
}

func scanQueues(rows pgx.Rows) ([]*queue.Queue, error) {
	var queues []*queue.Queue
	for rows.Next() {
		q, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		queues = append(queues, q)
	}
	return queues, rows.Err()
}
