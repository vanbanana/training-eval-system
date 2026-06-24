// Package middleware provides HTTP middleware (auth, CORS, rate limiting, etc.).
package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/smartedu/training-eval-system/internal/crypto"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	// ClaimsKey is the context key for JWT claims.
	ClaimsKey contextKey = "claims"
)

// GetClaims extracts JWT claims from the request context.
func GetClaims(ctx context.Context) *crypto.Claims {
	claims, _ := ctx.Value(ClaimsKey).(*crypto.Claims)
	return claims
}

// AuthMiddleware validates the JWT from the Authorization header and injects claims into context.
func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"detail":"Missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"detail":"Invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]
			claims, err := crypto.VerifyToken(jwtSecret, token)
			if err != nil {
				http.Error(w, `{"detail":"Invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Reject refresh tokens on protected routes (token-type enforcement)
			if claims.Type != "access" {
				http.Error(w, `{"detail":"Access token required"}`, http.StatusUnauthorized)
				return
			}

			// Inject claims into context
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that checks if the user has one of the allowed roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"detail":"Authentication required"}`, http.StatusUnauthorized)
				return
			}

			if !allowed[claims.Role] {
				http.Error(w, `{"detail":"Insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SessionTimeout returns middleware that rejects requests if the token was issued
// more than maxIdle ago (simulating session inactivity timeout).
// In practice, the frontend refreshes tokens on activity; this catches stale tokens.
func SessionTimeout(maxIdle time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check if token was issued too long ago (session idle check)
			issuedAt := time.Unix(claims.Iat, 0)
			if time.Since(issuedAt) > maxIdle {
				http.Error(w, `{"detail":"Session expired due to inactivity"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AccountLockout tracks failed login attempts per user and enforces lockout.
type AccountLockout struct {
	mu       sync.RWMutex
	attempts map[string]*lockoutEntry
	maxFails int
	lockDur  time.Duration
}

type lockoutEntry struct {
	count    int
	lockedAt time.Time
}

// NewAccountLockout creates a new lockout tracker.
// maxFails: number of failures before lockout (default 5).
// lockDuration: how long the lockout lasts (default 15 minutes).
func NewAccountLockout(maxFails int, lockDuration time.Duration) *AccountLockout {
	return &AccountLockout{
		attempts: make(map[string]*lockoutEntry),
		maxFails: maxFails,
		lockDur:  lockDuration,
	}
}

// IsLocked returns true if the given username is currently locked out.
func (al *AccountLockout) IsLocked(username string) bool {
	al.mu.RLock()
	defer al.mu.RUnlock()

	entry, ok := al.attempts[username]
	if !ok {
		return false
	}
	if entry.count < al.maxFails {
		return false
	}
	// Check if lock has expired
	if time.Since(entry.lockedAt) > al.lockDur {
		return false
	}
	return true
}

// RecordFailure increments the failure count for a username.
// Returns true if the account is now locked.
func (al *AccountLockout) RecordFailure(username string) bool {
	al.mu.Lock()
	defer al.mu.Unlock()

	entry, ok := al.attempts[username]
	if !ok {
		entry = &lockoutEntry{}
		al.attempts[username] = entry
	}

	// If previously locked and lock expired, reset
	if entry.count >= al.maxFails && time.Since(entry.lockedAt) > al.lockDur {
		entry.count = 0
	}

	entry.count++
	if entry.count >= al.maxFails {
		entry.lockedAt = time.Now()
		return true
	}
	return false
}

// RecordSuccess resets the failure count for a username.
func (al *AccountLockout) RecordSuccess(username string) {
	al.mu.Lock()
	defer al.mu.Unlock()
	delete(al.attempts, username)
}
