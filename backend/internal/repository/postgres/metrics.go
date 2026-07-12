package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	domainmetrics "github.com/your-org/job-scheduler/internal/domain/metrics"
	platformdb "github.com/your-org/job-scheduler/internal/platform/db"
)

type MetricsRepo struct {
	pool *platformdb.Pool
}

func NewMetricsRepo(pool *platformdb.Pool) *MetricsRepo {
	return &MetricsRepo{pool: pool}
}

func (r *MetricsRepo) SaveSnapshot(ctx context.Context, m *domainmetrics.QueueMetricsSnapshot) error {
	query := `
		INSERT INTO queue_metrics (
			id, queue_id, window_start, window_end, 
			jobs_enqueued, jobs_succeeded, jobs_failed, jobs_dead, 
			avg_wait_ms, avg_duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, query,
		m.ID, m.QueueID, m.WindowStart, m.WindowEnd,
		m.JobsEnqueued, m.JobsSucceeded, m.JobsFailed, m.JobsDead,
		m.AvgWaitMs, m.AvgDurationMs,
	)
	return err
}

func (r *MetricsRepo) GetSnapshots(ctx context.Context, queueID uuid.UUID, since time.Time) ([]domainmetrics.QueueMetricsSnapshot, error) {
	query := `
		SELECT id, queue_id, window_start, window_end, jobs_enqueued, jobs_succeeded, jobs_failed, jobs_dead, avg_wait_ms, avg_duration_ms
		FROM queue_metrics
		WHERE queue_id = $1 AND window_end >= $2
		ORDER BY window_start ASC
	`
	rows, err := r.pool.Query(ctx, query, queueID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []domainmetrics.QueueMetricsSnapshot
	for rows.Next() {
		var m domainmetrics.QueueMetricsSnapshot
		if err := rows.Scan(
			&m.ID, &m.QueueID, &m.WindowStart, &m.WindowEnd,
			&m.JobsEnqueued, &m.JobsSucceeded, &m.JobsFailed, &m.JobsDead,
			&m.AvgWaitMs, &m.AvgDurationMs,
		); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, m)
	}
	return snapshots, rows.Err()
}

func (r *MetricsRepo) GetSystemMetrics(ctx context.Context) (*domainmetrics.SystemMetrics, error) {
	m := &domainmetrics.SystemMetrics{}

	// Query workers
	err := r.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE status = 'draining')
		FROM workers
	`).Scan(&m.ActiveWorkers, &m.DrainingWorkers)
	if err != nil {
		return nil, err
	}

	// Query total queues
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM queues`).Scan(&m.TotalQueues)
	if err != nil {
		return nil, err
	}

	// Query jobs
	err = r.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'running')
		FROM jobs
	`).Scan(&m.TotalJobsPending, &m.TotalJobsRunning)
	if err != nil {
		return nil, err
	}

	return m, nil
}
