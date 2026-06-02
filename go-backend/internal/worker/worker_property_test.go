package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 10: Worker pool concurrency limit
func TestProperty_WorkerPoolConcurrencyLimit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		workerCount := rapid.IntRange(1, 4).Draw(t, "workerCount")
		taskCount := rapid.IntRange(workerCount+1, workerCount*3).Draw(t, "taskCount")

		pool := NewPool(workerCount, taskCount)
		defer pool.Shutdown()

		var concurrent atomic.Int32
		var maxConcurrent atomic.Int32

		done := make(chan struct{})
		submitted := 0

		for i := 0; i < taskCount; i++ {
			task := &Task{
				ID: "task",
				Fn: func(ctx context.Context) error {
					cur := concurrent.Add(1)
					// Track max concurrent
					for {
						old := maxConcurrent.Load()
						if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
							break
						}
					}
					time.Sleep(10 * time.Millisecond)
					concurrent.Add(-1)
					return nil
				},
			}
			if err := pool.Submit(task); err == nil {
				submitted++
			}
		}

		// Wait for tasks to complete
		time.Sleep(time.Duration(taskCount/workerCount+2) * 50 * time.Millisecond)
		close(done)

		max := int(maxConcurrent.Load())
		if max > workerCount {
			t.Fatalf("max concurrent %d exceeded worker count %d", max, workerCount)
		}
	})
}

// Property 11: Task panic recovery
func TestProperty_TaskPanicRecovery(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pool := NewPool(2, 10)

		// Submit a panicking task
		panicTask := &Task{
			ID: "panic-task",
			Fn: func(ctx context.Context) error {
				panic("test panic")
			},
		}
		_ = pool.Submit(panicTask)

		// Submit a normal task after — pool should still work
		normalDone := make(chan bool, 1)
		normalTask := &Task{
			ID: "normal-task",
			Fn: func(ctx context.Context) error {
				normalDone <- true
				return nil
			},
		}
		if err := pool.Submit(normalTask); err != nil {
			t.Fatalf("Submit after panic failed: %v", err)
		}

		// The key property: pool recovers and processes subsequent tasks
		select {
		case <-normalDone:
			// Success — pool recovered from panic
		case <-time.After(15 * time.Second):
			t.Fatal("normal task not executed after panic — pool did not recover")
		}

		pool.Shutdown()
	})
}
