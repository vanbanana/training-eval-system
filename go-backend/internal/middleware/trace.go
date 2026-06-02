package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	// TraceIDKey is the context key for the trace ID.
	TraceIDKey contextKey = "trace_id"
	// TraceIDHeader is the HTTP header name for trace ID propagation.
	TraceIDHeader = "X-Trace-ID"
)

// GetTraceID extracts the trace ID from the request context.
func GetTraceID(ctx context.Context) string {
	id, _ := ctx.Value(TraceIDKey).(string)
	return id
}

// TraceMiddleware generates or inherits a trace_id for each request.
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get(TraceIDHeader)
		if traceID == "" {
			traceID = generateTraceID()
		}

		// Set response header
		w.Header().Set(TraceIDHeader, traceID)

		// Inject into context
		ctx := context.WithValue(r.Context(), TraceIDKey, traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
