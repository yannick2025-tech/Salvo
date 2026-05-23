// Package pool provides a fixed-size goroutine pool for driving test
// scenario execution in Salvo.
//
// The pool supports two run modes:
//   - RunModeCount:  execute a fixed number of tasks then stop.
//   - RunModeDuration: execute tasks for a fixed wall-clock duration.
//
// Tasks are submitted via Submit and the pool blocks on Wait until the
// configured termination condition is met or the parent context is
// cancelled.
package pool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// RunMode determines how the pool terminates a test run.
type RunMode int

const (
	// RunModeCount runs the pool until exactly Count tasks have been
	// submitted and completed.
	RunModeCount RunMode = iota
	// RunModeDuration runs the pool for the specified Duration.
	RunModeDuration
)

// Task is a unit of work submitted to the pool. Implementations must be
// safe for concurrent use.
type Task func(ctx context.Context) error

// Config controls the pool's termination behaviour.
type Config struct {
	// RunMode selects between count-based and duration-based termination.
	RunMode RunMode
	// Count is the total number of tasks when RunMode is RunModeCount.
	Count int64
	// Duration is the total run time when RunMode is RunModeDuration.
	Duration time.Duration
}

// Validate checks the Config for semantic errors.
func (c Config) Validate() error {
	switch c.RunMode {
	case RunModeCount:
		if c.Count <= 0 {
			return fmt.Errorf("pool count must be > 0, got %d", c.Count)
		}
	case RunModeDuration:
		if c.Duration <= 0 {
			return fmt.Errorf("pool duration must be > 0, got %s", c.Duration)
		}
	default:
		return fmt.Errorf("invalid pool run_mode: %d", c.RunMode)
	}
	return nil
}

// Pool is a fixed-size goroutine pool that executes submitted tasks
// respecting a configurable termination condition.
type Pool struct {
	workers     int
	cfg         Config
	tasks       chan Task
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	waitTracker *WaitTimeTracker

	submitted atomic.Int64
	completed atomic.Int64
	shutdown  atomic.Bool
	firstErr  error
	errOnce   sync.Once
}

// New creates a Pool with the given number of workers and configuration.
// Returns an error if workers <= 0 or the config is invalid.
func New(workers int, cfg Config) (*Pool, error) {
	return NewWithContext(context.Background(), workers, cfg)
}

// NewWithContext creates a Pool that respects the provided parent context
// for external cancellation. Returns an error if workers <= 0 or the
// config is invalid.
func NewWithContext(parent context.Context, workers int, cfg Config) (*Pool, error) {
	if workers <= 0 {
		return nil, fmt.Errorf("worker count must be > 0, got %d", workers)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(parent)

	p := &Pool{
		workers:     workers,
		cfg:         cfg,
		tasks:       make(chan Task, workers*2),
		ctx:         ctx,
		cancel:      cancel,
		waitTracker: NewWaitTimeTracker(10000),
	}

	p.start()
	return p, nil
}

// start launches the worker goroutines and, for duration mode, a timer
// that cancels the pool context when the duration elapses.
func (p *Pool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	if p.cfg.RunMode == RunModeDuration {
		go func() {
			timer := time.NewTimer(p.cfg.Duration)
			defer timer.Stop()
			select {
			case <-timer.C:
				p.cancel()
			case <-p.ctx.Done():
			}
		}()
	}
}

// worker reads tasks from the channel and executes them until the
// context is cancelled.
func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case task := <-p.tasks:
			if err := task(p.ctx); err != nil {
				p.errOnce.Do(func() {
					p.firstErr = err
					p.cancel()
				})
			}
			p.completed.Add(1)
		}
	}
}

// Submit enqueues a task for execution. It panics if called after
// Shutdown.
func (p *Pool) Submit(task Task) {
	if p.shutdown.Load() {
		panic("pool: submit after shutdown")
	}
	p.submitted.Add(1)

	enqueuedAt := time.Now()

	select {
	case p.tasks <- func(ctx context.Context) error {
		if p.ctx.Err() == nil {
			p.waitTracker.Record(time.Since(enqueuedAt))
		}
		return task(ctx)
	}:
	case <-p.ctx.Done():
	}
}

// Wait blocks until the pool's termination condition is met (all count
// tasks completed, duration elapsed, or context cancelled) and all
// workers have exited. Returns the first task error encountered, or nil.
func (p *Pool) Wait() error {
	switch p.cfg.RunMode {
	case RunModeCount:
		p.waitCount()
	case RunModeDuration:
		<-p.ctx.Done()
	}

	p.cancel()
	p.wg.Wait()

	return p.firstErr
}

// waitCount polls until all submitted tasks have completed or the
// context is cancelled.
func (p *Pool) waitCount() {
	for {
		if p.ctx.Err() != nil {
			return
		}
		if p.completed.Load() >= p.submitted.Load() && p.submitted.Load() >= p.cfg.Count {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// Shutdown marks the pool as shut down and cancels all pending work.
// After Shutdown, Submit will panic.
func (p *Pool) Shutdown() {
	p.shutdown.Store(true)
	p.cancel()
}

// WorkerCount returns the number of worker goroutines.
func (p *Pool) WorkerCount() int {
	return p.workers
}

// Submitted returns the total number of tasks submitted to the pool.
func (p *Pool) Submitted() int64 {
	return p.submitted.Load()
}

// Completed returns the total number of tasks that finished execution.
func (p *Pool) Completed() int64 {
	return p.completed.Load()
}

// TaskWaitStats returns aggregated wait-time statistics for tasks in
// the pool. The statistics reflect the time between task submission
// (Submit) and actual execution start.
func (p *Pool) TaskWaitStats() WaitTimeStats {
	return p.waitTracker.Stats()
}

// ActiveWorkers returns the number of currently executing workers,
// computed as submitted minus completed tasks (capped at worker count).
func (p *Pool) ActiveWorkers() int {
	active := int(p.submitted.Load() - p.completed.Load())
	if active < 0 {
		active = 0
	}
	if active > p.workers {
		active = p.workers
	}
	return active
}

// PendingQueueLen returns the number of tasks waiting in the queue.
func (p *Pool) PendingQueueLen() int {
	return len(p.tasks)
}
