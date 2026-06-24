// Package handler — StreamTracker manages per-user and global concurrent LLM stream limits.
package handler

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// StreamTracker tracks concurrent agent streams per user and globally.
// It is safe for concurrent use.
type StreamTracker struct {
	mu        sync.Mutex
	active    map[int64]int // userID → active stream count
	global    atomic.Int64  // global active stream count
	userMax   int           // per-user concurrent limit
	globalMax int           // global concurrent limit
}

// NewStreamTracker creates a tracker with the given per-user and global limits.
func NewStreamTracker(userMax, globalMax int) *StreamTracker {
	if userMax <= 0 {
		userMax = 2
	}
	if globalMax <= 0 {
		globalMax = 50
	}
	return &StreamTracker{
		active:    make(map[int64]int),
		userMax:   userMax,
		globalMax: globalMax,
	}
}

// Acquire tries to reserve a stream slot for the given user.
// Returns nil on success, or an error if the limit is exceeded.
func (t *StreamTracker) Acquire(userID int64) error {
	// Check global limit first (fast path, atomic)
	if int(t.global.Load()) >= t.globalMax {
		return fmt.Errorf("global concurrent stream limit reached (%d)", t.globalMax)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	current := t.active[userID]
	if current >= t.userMax {
		return fmt.Errorf("user concurrent stream limit reached (%d)", t.userMax)
	}

	t.active[userID] = current + 1
	t.global.Add(1)
	return nil
}

// Release frees a stream slot for the given user.
// Must be called exactly once per successful Acquire.
func (t *StreamTracker) Release(userID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.active[userID] > 0 {
		t.active[userID]--
		if t.active[userID] == 0 {
			delete(t.active, userID)
		}
		t.global.Add(-1)
	}
}

// UserActive returns the number of active streams for a given user.
func (t *StreamTracker) UserActive(userID int64) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active[userID]
}

// GlobalActive returns the total number of active streams across all users.
func (t *StreamTracker) GlobalActive() int64 {
	return t.global.Load()
}
