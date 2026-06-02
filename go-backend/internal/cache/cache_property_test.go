package cache

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 28: LRU cache TTL expiration
func TestProperty_LRUCacheTTLExpiration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ttl := 50 * time.Millisecond
		c := New[string, int](100, ttl)

		key := rapid.StringMatching(`[a-z]{3,10}`).Draw(t, "key")
		value := rapid.IntRange(1, 1000).Draw(t, "value")

		c.Set(key, value)

		// Should be retrievable immediately
		got, ok := c.Get(key)
		if !ok {
			t.Fatal("key should exist immediately after Set")
		}
		if got != value {
			t.Fatalf("value mismatch: got %d, want %d", got, value)
		}

		// Wait for TTL to expire
		time.Sleep(ttl + 10*time.Millisecond)

		// Should be expired
		_, ok = c.Get(key)
		if ok {
			t.Fatal("key should be expired after TTL")
		}
	})
}

// Property 28 supplement: LRU eviction
func TestProperty_LRUEviction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		capacity := rapid.IntRange(2, 10).Draw(t, "capacity")
		c := New[int, int](capacity, 1*time.Hour)

		// Fill cache beyond capacity
		for i := 0; i < capacity+1; i++ {
			c.Set(i, i*10)
		}

		// Cache should not exceed capacity
		if c.Len() > capacity {
			t.Fatalf("cache length %d exceeds capacity %d", c.Len(), capacity)
		}

		// The first item (LRU) should have been evicted
		_, ok := c.Get(0)
		if ok {
			t.Fatal("first item should have been evicted (LRU)")
		}

		// The last item should still be present
		got, ok := c.Get(capacity)
		if !ok {
			t.Fatal("last item should still be present")
		}
		if got != capacity*10 {
			t.Fatalf("value mismatch: got %d, want %d", got, capacity*10)
		}
	})
}
