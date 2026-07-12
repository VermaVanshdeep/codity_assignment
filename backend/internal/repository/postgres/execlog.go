package postgres

import (
	"context"

	"github.com/google/uuid"
	domainexeclog "github.com/your-org/job-scheduler/internal/domain/execlog"
	platformdb "github.com/your-org/job-scheduler/internal/platform/db"
)

type ExecLogRepo struct {
	pool *platformdb.Pool
}

func NewExecLogRepo(pool *platformdb.Pool) *ExecLogRepo {
	return &ExecLogRepo{pool: pool}
}

func (r *ExecLogRepo) CreateExecution(ctx context.Context, e *domainexeclog.JobExecution) error {
	query := `
		INSERT INTO job_executions (id, job_id, worker_id, attempt, status, started_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, e.ID, e.JobID, e.WorkerID, e.Attempt, e.Status, e.StartedAt)
	return err
}

func (r *ExecLogRepo) UpdateExecution(ctx context.Context, e *domainexeclog.JobExecution) error {
	query := `
		UPDATE job_executions
		SET status = $1, completed_at = $2, duration_ms = $3, error_message = $4, stack_trace = $5, result = $6
		WHERE id = $7
	`
	_, err := r.pool.Exec(ctx, query, e.Status, e.CompletedAt, e.DurationMs, e.ErrorMessage, e.StackTrace, e.Result, e.ID)
	return err
}

func (r *ExecLogRepo) ListExecutionsByJob(ctx context.Context, jobID uuid.UUID) ([]domainexeclog.JobExecution, error) {
	query := `
		SELECT id, job_id, worker_id, attempt, status, started_at, completed_at, duration_ms, error_message, stack_trace, result
		FROM job_executions
		WHERE job_id = $1
		ORDER BY attempt ASC
	`
	rows, err := r.pool.Query(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var execs []domainexeclog.JobExecution
	for rows.Next() {
		var e domainexeclog.JobExecution
		if err := rows.Scan(
			&e.ID, &e.JobID, &e.WorkerID, &e.Attempt, &e.Status, &e.StartedAt,
			&e.CompletedAt, &e.DurationMs, &e.ErrorMessage, &e.StackTrace, &e.Result,
		); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

func (r *ExecLogRepo) AddLogs(ctx context.Context, logs []domainexeclog.ExecutionLog) error {
	if len(logs) == 0 {
		return nil
	}
	// Simplified single insert for this iteration
	// In production, pgx.CopyFrom is preferred for bulk inserts
	for _, l := range logs {
		query := `
			INSERT INTO execution_logs (id, execution_id, level, message, metadata, logged_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
		if _, err := r.pool.Exec(ctx, query, l.ID, l.ExecutionID, l.Level, l.Message, l.Metadata, l.LoggedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *ExecLogRepo) ListLogsByExecution(ctx context.Context, executionID uuid.UUID) ([]domainexeclog.ExecutionLog, error) {
	query := `
		SELECT id, execution_id, level, message, metadata, logged_at
		FROM execution_logs
		WHERE execution_id = $1
		ORDER BY logged_at ASC
	`
	rows, err := r.pool.Query(ctx, query, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domainexeclog.ExecutionLog
	for rows.Next() {
		var l domainexeclog.ExecutionLog
		if err := rows.Scan(&l.ID, &l.ExecutionID, &l.Level, &l.Message, &l.Metadata, &l.LoggedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (r *ExecLogRepo) ListLogsByJob(ctx context.Context, jobID uuid.UUID) ([]domainexeclog.ExecutionLog, error) {
	query := `
		SELECT el.id, el.execution_id, el.level, el.message, el.metadata, el.logged_at
		FROM execution_logs el
		JOIN job_executions je ON el.execution_id = je.id
		WHERE je.job_id = $1
		ORDER BY el.logged_at ASC
	`
	rows, err := r.pool.Query(ctx, query, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domainexeclog.ExecutionLog
	for rows.Next() {
		var l domainexeclog.ExecutionLog
		if err := rows.Scan(&l.ID, &l.ExecutionID, &l.Level, &l.Message, &l.Metadata, &l.LoggedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
