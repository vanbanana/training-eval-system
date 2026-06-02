// Package cache provides an in-process LRU cache with TTL support.
package cache

import (
	"sync"
	"time"
)

// entry holds a cached value with its expiration time.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// LRU is a generic thread-safe LRU cache with TTL expiration.
type LRU[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]*entry[V]
	order    []K // most recently used at the end
	capacity int
	ttl      time.Duration
	done     chan struct{}
}

// New creates a new LRU cache with the given capacity and TTL.
func New[K comparable, V any](capacity int, ttl time.Duration) *LRU[K, V] {
	return &LRU[K, V]{
		items:    make(map[K]*entry[V], capacity),
		order:    make([]K, 0, capacity),
		capacity: capacity,
		ttl:      ttl,
		done:     make(chan struct{}),
	}
}

// Get retrieves a value from the cache. Returns the value and true if found and not expired.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	// Check TTL
	if time.Now().After(e.expiresAt) {
		c.removeLocked(key)
		var zero V
		return zero, false
	}

	// Move to end (most recently used)
	c.moveToEnd(key)
	return e.value, true
}

// Set adds or updates a value in the cache.
func (c *LRU[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[key]; ok {
		// Update existing
		c.items[key] = &entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
		c.moveToEnd(key)
		return
	}

	// Evict if at capacity
	if len(c.items) >= c.capacity && len(c.order) > 0 {
		oldest := c.order[0]
		c.removeLocked(oldest)
	}

	c.items[key] = &entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.order = append(c.order, key)
}

// Delete removes a key from the cache.
func (c *LRU[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(key)
}

// Len returns the number of items in the cache.
func (c *LRU[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Cleanup removes all expired entries.
func (c *LRU[K, V]) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, e := range c.items {
		if now.After(e.expiresAt) {
			c.removeLocked(key)
		}
	}
}

// StartCleanup starts a background goroutine that periodically removes expired entries.
// Call Stop() to terminate the cleanup goroutine.
func (c *LRU[K, V]) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Cleanup()
			case <-c.done:
				return
			}
		}
	}()
}

// Stop terminates the background cleanup goroutine.
func (c *LRU[K, V]) Stop() {
	close(c.done)
}

func (c *LRU[K, V]) removeLocked(key K) {
	delete(c.items, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *LRU[K, V]) moveToEnd(key K) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			break
		}
	}
}
