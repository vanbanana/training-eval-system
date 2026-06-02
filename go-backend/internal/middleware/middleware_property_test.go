package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"pgregory.net/rapid"
)

// Property 7: RBAC enforcement
func TestProperty_RBACEnforcement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		allowedRoles := rapid.SliceOfNDistinct(
			rapid.SampledFrom([]string{"admin", "teacher", "student"}),
			1, 3, rapid.ID[string],
		).Draw(t, "allowedRoles")

		userRole := rapid.SampledFrom([]string{"admin", "teacher", "student"}).Draw(t, "userRole")

		// Check if userRole is in allowedRoles
		isAllowed := false
		for _, r := range allowedRoles {
			if r == userRole {
				isAllowed = true
				break
			}
		}

		// Create a handler chain with RequireRole
		handler := RequireRole(allowedRoles...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Create request with claims in context
		req := httptest.NewRequest("GET", "/test", nil)
		claims := &crypto.Claims{Sub: 1, Username: "test", Role: userRole, Type: "access", Iat: time.Now().Unix(), Exp: time.Now().Add(time.Hour).Unix()}
		ctx := req.Context()
		ctx = setClaimsContext(ctx, claims)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if isAllowed && rr.Code != http.StatusOK {
			t.Fatalf("role %q should be allowed (allowed=%v), got status %d", userRole, allowedRoles, rr.Code)
		}
		if !isAllowed && rr.Code != http.StatusForbidden {
			t.Fatalf("role %q should be forbidden (allowed=%v), got status %d", userRole, allowedRoles, rr.Code)
		}
	})
}

// Property 8: Account lockout threshold
func TestProperty_AccountLockout(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxFails := 5
		lockDur := 15 * time.Minute
		al := NewAccountLockout(maxFails, lockDur)

		username := rapid.StringMatching(`[a-z]{5,10}`).Draw(t, "username")
		failures := rapid.IntRange(0, 10).Draw(t, "failures")

		for i := 0; i < failures; i++ {
			al.RecordFailure(username)
		}

		isLocked := al.IsLocked(username)

		if failures >= maxFails && !isLocked {
			t.Fatalf("should be locked after %d failures (max=%d)", failures, maxFails)
		}
		if failures < maxFails && isLocked {
			t.Fatalf("should NOT be locked after %d failures (max=%d)", failures, maxFails)
		}
	})
}

// Property 9: Session timeout enforcement
func TestProperty_SessionTimeout(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxIdle := 30 * time.Minute
		// Generate a time offset in minutes
		minutesAgo := rapid.IntRange(0, 60).Draw(t, "minutesAgo")

		iat := time.Now().Add(-time.Duration(minutesAgo) * time.Minute)

		claims := &crypto.Claims{
			Sub:      1,
			Username: "test",
			Role:     "student",
			Type:     "access",
			Iat:      iat.Unix(),
			Exp:      time.Now().Add(time.Hour).Unix(),
		}

		// Create handler with session timeout
		handler := SessionTimeout(maxIdle)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		ctx := setClaimsContext(req.Context(), claims)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		shouldExpire := minutesAgo > 30
		if shouldExpire && rr.Code == http.StatusOK {
			t.Fatalf("session should be expired after %d minutes (max=30)", minutesAgo)
		}
		if !shouldExpire && rr.Code != http.StatusOK {
			// Allow borderline case (exactly 30 minutes) to go either way due to timing
			if minutesAgo == 30 {
				return // borderline, skip
			}
			t.Fatalf("session should be valid after %d minutes (max=30), got %d", minutesAgo, rr.Code)
		}
	})
}

// Property 15: Rate limiting enforcement
func TestProperty_RateLimiting(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		limit := 5
		window := 1 * time.Minute
		rl := NewRateLimiter(limit, window)

		requests := rapid.IntRange(1, 15).Draw(t, "requests")

		handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		var lastStatus int
		for i := 0; i < requests; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			lastStatus = rr.Code
		}

		if requests > limit && lastStatus != http.StatusTooManyRequests {
			t.Fatalf("should be rate limited after %d requests (limit=%d), got %d", requests, limit, lastStatus)
		}
		if requests <= limit && lastStatus != http.StatusOK {
			t.Fatalf("should NOT be rate limited after %d requests (limit=%d), got %d", requests, limit, lastStatus)
		}
	})
}

// Property 18: Trace ID uniqueness
func TestProperty_TraceIDUniqueness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(10, 100).Draw(t, "n")
		ids := make(map[string]bool, n)

		handler := TraceMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := GetTraceID(r.Context())
			if traceID == "" {
				t.Fatal("trace_id should not be empty")
			}
			ids[traceID] = true
		}))

		for i := 0; i < n; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}

		if len(ids) != n {
			t.Fatalf("expected %d unique trace IDs, got %d", n, len(ids))
		}
	})
}

// helper to set claims in context
func setClaimsContext(ctx context.Context, claims *crypto.Claims) context.Context {
	return context.WithValue(ctx, ClaimsKey, claims)
}
