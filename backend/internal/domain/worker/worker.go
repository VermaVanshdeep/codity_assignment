package worker

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDraining Status = "draining"
	StatusDead     Status = "dead"
)

type Worker struct {
	ID              uuid.UUID
	Hostname        string
	PID             int
	Version         string
	Status          Status
	Concurrency     int
	Queues          []string
	LastHeartbeatAt time.Time
	RegisteredAt    time.Time
}

type Repository interface {
	Register(ctx context.Context, w *Worker) error
	Heartbeat(ctx context.Context, id uuid.UUID) error
}
