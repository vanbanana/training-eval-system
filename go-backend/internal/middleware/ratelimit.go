package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple sliding window rate limiter per IP.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    int
	window   time.Duration
	done     chan struct{}
}

type visitor struct {
	timestamps []time.Time
}

// NewRateLimiter creates a rate limiter that allows `limit` requests per `window` per IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
		done:     make(chan struct{}),
	}
	// Start cleanup goroutine
	go rl.cleanup()
	return rl
}

// Stop terminates the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

// Middleware returns an HTTP middleware that enforces rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"detail":"Rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, ok := rl.visitors[ip]
	if !ok {
		v = &visitor{}
		rl.visitors[ip] = v
	}

	// Remove timestamps outside the window
	cutoff := now.Add(-rl.window)
	valid := v.timestamps[:0]
	for _, ts := range v.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	v.timestamps = valid

	if len(v.timestamps) >= rl.limit {
		return false
	}

	v.timestamps = append(v.timestamps, now)
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, v := range rl.visitors {
				cutoff := now.Add(-rl.window)
				valid := v.timestamps[:0]
				for _, ts := range v.timestamps {
					if ts.After(cutoff) {
						valid = append(valid, ts)
					}
				}
				v.timestamps = valid
				if len(v.timestamps) == 0 {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP)
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr (strip port)
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
