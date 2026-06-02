package llm

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	StateClosed   CircuitState = iota // normal operation
	StateOpen                         // rejecting requests
	StateHalfOpen                     // testing recovery
)

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failures     int
	maxFailures  int
	cooldown     time.Duration
	lastFailedAt time.Time
}

// NewCircuitBreaker creates a circuit breaker.
// maxFailures: consecutive failures before opening (default 5).
// cooldown: time to wait before half-open (default 30s).
func NewCircuitBreaker(maxFailures int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		cooldown:    cooldown,
	}
}

// Allow checks if a request is allowed through the circuit breaker.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(cb.lastFailedAt) > cb.cooldown {
			cb.state = StateHalfOpen
			return nil
		}
		return fmt.Errorf("circuit breaker is open")
	case StateHalfOpen:
		return nil
	}
	return nil
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = StateClosed
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailedAt = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
