// Package worker implements a goroutine-based worker pool with buffered channels.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Pool manages a fixed number of worker goroutines processing tasks from a buffered channel.
type Pool struct {
	tasks      chan *Task
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	maxRetries int
	workerCnt  int
}

// NewPool creates a worker pool with the given number of workers and task buffer size.
func NewPool(workerCount, bufferSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		tasks:      make(chan *Task, bufferSize),
		ctx:        ctx,
		cancel:     cancel,
		maxRetries: 3,
		workerCnt:  workerCount,
	}

	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	slog.Info("worker pool started", "workers", workerCount, "buffer", bufferSize)
	return p
}

// Submit adds a task to the pool's queue. Returns error if pool is shutting down or queue is full.
func (p *Pool) Submit(task *Task) error {
	task.Status = StatusPending
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("worker: pool is shut down")
	case p.tasks <- task:
		return nil
	default:
		return fmt.Errorf("worker: task queue full")
	}
}

// Shutdown gracefully stops the pool, waiting up to 30 seconds for in-flight tasks.
func (p *Pool) Shutdown() {
	slog.Info("worker pool shutting down")
	p.cancel()
	close(p.tasks)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("worker pool drained")
	case <-time.After(30 * time.Second):
		slog.Warn("worker pool drain timeout (30s)")
	}
}

// WorkerCount returns the number of workers in the pool.
func (p *Pool) WorkerCount() int {
	return p.workerCnt
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for task := range p.tasks {
		p.executeWithRetry(id, task)
	}
}

func (p *Pool) executeWithRetry(workerID int, task *Task) {
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-p.ctx.Done():
				task.Status = StatusFailed
				task.Err = fmt.Errorf("worker: pool shutdown during retry")
				return
			}
		}

		task.Status = StatusRunning
		err := p.safeExecute(task)

		if err == nil {
			task.Status = StatusCompleted
			return
		}

		task.Err = err
		task.Retries = attempt + 1

		if attempt < p.maxRetries {
			slog.Warn("task failed, retrying",
				"task_id", task.ID,
				"worker", workerID,
				"attempt", attempt+1,
				"error", err.Error(),
			)
		}
	}

	task.Status = StatusFailed
	slog.Error("task failed permanently",
		"task_id", task.ID,
		"retries", task.Retries,
		"error", task.Err.Error(),
	)
}

// safeExecute runs the task function with panic recovery.
func (p *Pool) safeExecute(task *Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("worker: panic recovered: %v", r)
		}
	}()

	return task.Fn(p.ctx)
}
