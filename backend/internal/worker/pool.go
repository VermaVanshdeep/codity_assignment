package worker

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/your-org/job-scheduler/internal/domain/job"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Pool manages a fixed number of goroutines that execute claimed jobs.
type Pool struct {
	concurrency int
	jobsChan    chan *job.Job
	executor    *Executor
	log         *logger.Logger
	wg          sync.WaitGroup
	active      int64 // atomic counter for currently executing jobs
}

// NewPool creates a new worker pool.
func NewPool(concurrency int, executor *Executor, log *logger.Logger) *Pool {
	return &Pool{
		concurrency: concurrency,
		jobsChan:    make(chan *job.Job, concurrency), // buffered channel up to concurrency size
		executor:    executor,
		log:         log.WithField("component", "pool"),
	}
}

// Start launches the worker goroutines.
func (p *Pool) Start(ctx context.Context) {
	p.log.Info("starting worker pool", logger.Int("concurrency", p.concurrency))

	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}
}

// Stop gracefully drains the channel and waits for active jobs to finish.
func (p *Pool) Stop() {
	p.log.Info("stopping worker pool, waiting for active jobs to finish...")
	close(p.jobsChan) // signal workers to exit when channel is empty
	p.wg.Wait()
	p.log.Info("worker pool stopped cleanly")
}

// Submit enqueues a job for execution. This blocks if the pool is full.
func (p *Pool) Submit(j *job.Job) {
	p.jobsChan <- j
}

// AvailableWorkers returns the number of idle worker goroutines.
func (p *Pool) AvailableWorkers() int {
	active := int(atomic.LoadInt64(&p.active))
	return p.concurrency - active
}

func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	for j := range p.jobsChan {
		atomic.AddInt64(&p.active, 1)

		// Create a detached context for execution so that cancellation of the
		// global pool context doesn't abruptly kill the executing job.
		// Graceful shutdown relies on waiting for these to finish.
		execCtx := context.Background()
		p.executor.Execute(execCtx, j)

		atomic.AddInt64(&p.active, -1)
	}
}
