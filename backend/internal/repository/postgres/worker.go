package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/domain/worker"
	platformdb "github.com/your-org/job-scheduler/internal/platform/db"
)

type WorkerRepo struct {
	pool *platformdb.Pool
}

func NewWorkerRepo(pool *platformdb.Pool) *WorkerRepo {
	return &WorkerRepo{pool: pool}
}

func (r *WorkerRepo) Register(ctx context.Context, w *worker.Worker) error {
	query := `
		INSERT INTO workers (id, hostname, pid, version, status, concurrency, queues, last_heartbeat_at, registered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			last_heartbeat_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query,
		w.ID,
		w.Hostname,
		w.PID,
		w.Version,
		w.Status,
		w.Concurrency,
		w.Queues,
	)
	return err
}

func (r *WorkerRepo) Heartbeat(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE workers 
		SET last_heartbeat_at = NOW(), status = $2
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, worker.StatusActive)
	return err
}
