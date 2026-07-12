package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	domainbatch "github.com/your-org/job-scheduler/internal/domain/batch"
	platformdb "github.com/your-org/job-scheduler/internal/platform/db"
)

type BatchRepo struct {
	pool *platformdb.Pool
}

func NewBatchRepo(pool *platformdb.Pool) *BatchRepo {
	return &BatchRepo{pool: pool}
}

func (r *BatchRepo) Create(ctx context.Context, b *domainbatch.Batch) error {
	query := `
		INSERT INTO batches (id, queue_id, name, total_jobs, pending_count, succeeded_count, failed_count, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		b.ID, b.QueueID, b.Name, b.TotalJobs, b.PendingCount, b.SucceededCount, b.FailedCount, b.Status, b.CreatedAt, b.UpdatedAt,
	)
	return err
}

func (r *BatchRepo) Get(ctx context.Context, id uuid.UUID) (*domainbatch.Batch, error) {
	query := `
		SELECT id, queue_id, name, total_jobs, pending_count, succeeded_count, failed_count, status, created_at, updated_at
		FROM batches
		WHERE id = $1
	`
	b := &domainbatch.Batch{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.QueueID, &b.Name, &b.TotalJobs, &b.PendingCount, &b.SucceededCount, &b.FailedCount, &b.Status, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("batch not found")
		}
		return nil, err
	}
	return b, nil
}

func (r *BatchRepo) IncrementCounters(ctx context.Context, id uuid.UUID, succeeded int, failed int) error {
	query := `
		UPDATE batches
		SET succeeded_count = succeeded_count + $1,
			failed_count = failed_count + $2,
			pending_count = pending_count - ($1 + $2),
			status = CASE
				WHEN (succeeded_count + $1 + failed_count + $2) = total_jobs AND (failed_count + $2) = 0 THEN 'completed'::batch_status
				WHEN (succeeded_count + $1 + failed_count + $2) = total_jobs AND (failed_count + $2) > 0 THEN 'partial_failure'::batch_status
				ELSE 'running'::batch_status
			END,
			updated_at = $3
		WHERE id = $4
	`
	_, err := r.pool.Exec(ctx, query, succeeded, failed, time.Now().UTC(), id)
	return err
}
