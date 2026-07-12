package metrics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// QueueMetricsSnapshot represents aggregated stats for a queue over a time window.
type QueueMetricsSnapshot struct {
	ID            uuid.UUID
	QueueID       uuid.UUID
	WindowStart   time.Time
	WindowEnd     time.Time
	JobsEnqueued  int
	JobsSucceeded int
	JobsFailed    int
	JobsDead      int
	AvgWaitMs     int
	AvgDurationMs int
}

// SystemMetrics represents live, point-in-time metrics for the whole system.
type SystemMetrics struct {
	ActiveWorkers    int
	DrainingWorkers  int
	TotalQueues      int
	TotalJobsPending int
	TotalJobsRunning int
}

// Repository defines data access for metrics.
type Repository interface {
	SaveSnapshot(ctx context.Context, m *QueueMetricsSnapshot) error
	GetSnapshots(ctx context.Context, queueID uuid.UUID, since time.Time) ([]QueueMetricsSnapshot, error)
	GetSystemMetrics(ctx context.Context) (*SystemMetrics, error)
}
