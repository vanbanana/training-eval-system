package worker

import "context"

// TaskStatus represents the state of a worker task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// Task represents a unit of work to be executed by the worker pool.
type Task struct {
	ID      string
	Fn      func(ctx context.Context) error
	Status  TaskStatus
	Err     error
	Retries int
}
