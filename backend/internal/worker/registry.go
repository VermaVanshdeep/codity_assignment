// Package worker provides the worker runtime: goroutine pool, dispatcher,
// executor, heartbeat, reaper, and graceful shutdown.
package worker

import (
	"context"
	"fmt"

	"github.com/your-org/job-scheduler/internal/domain/job"
)

// HandlerFunc is the signature for a job handler function.
// Each job type (e.g., "email.send") maps to exactly one HandlerFunc.
// Returning an error causes the job to be retried (if retries remain) or moved to DLQ.
type HandlerFunc func(ctx context.Context, j *job.Job) error

// Registry maps job type strings to handler functions.
// It is populated at worker startup before the pool begins dispatching.
type Registry struct {
	handlers map[string]HandlerFunc
}

// NewRegistry creates an empty handler registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]HandlerFunc)}
}

// Register adds a handler for the given job type.
// Panics if the same type is registered twice — catch this at startup.
func (r *Registry) Register(jobType string, fn HandlerFunc) {
	if _, exists := r.handlers[jobType]; exists {
		panic(fmt.Sprintf("worker: handler for job type %q already registered", jobType))
	}
	r.handlers[jobType] = fn
}

// Get returns the handler for a job type, or an error if not registered.
func (r *Registry) Get(jobType string) (HandlerFunc, error) {
	fn, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for job type %q", jobType)
	}
	return fn, nil
}

// Types returns all registered job type names.
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}
