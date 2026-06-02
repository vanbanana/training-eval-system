package llm

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 13: Circuit breaker state transitions
func TestProperty_CircuitBreakerTransitions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxFailures := 5
		cooldown := 50 * time.Millisecond // short for testing
		cb := NewCircuitBreaker(maxFailures, cooldown)

		failures := rapid.IntRange(0, 10).Draw(t, "failures")

		// Record failures
		for i := 0; i < failures; i++ {
			cb.RecordFailure()
		}

		if failures >= maxFailures {
			// Should be open
			if cb.State() != StateOpen {
				t.Fatalf("after %d failures, state should be Open, got %v", failures, cb.State())
			}
			// Allow should fail
			if err := cb.Allow(); err == nil {
				t.Fatal("Allow should fail when circuit is open")
			}

			// Wait for cooldown
			time.Sleep(cooldown + 10*time.Millisecond)

			// Should transition to half-open on next Allow
			if err := cb.Allow(); err != nil {
				t.Fatalf("Allow should succeed after cooldown: %v", err)
			}
			if cb.State() != StateHalfOpen {
				t.Fatalf("state should be HalfOpen after cooldown, got %v", cb.State())
			}

			// Success should close it
			cb.RecordSuccess()
			if cb.State() != StateClosed {
				t.Fatalf("state should be Closed after success, got %v", cb.State())
			}
		} else {
			// Should still be closed
			if cb.State() != StateClosed {
				t.Fatalf("after %d failures (max=%d), state should be Closed, got %v", failures, maxFailures, cb.State())
			}
			if err := cb.Allow(); err != nil {
				t.Fatalf("Allow should succeed when closed: %v", err)
			}
		}
	})
}
