package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer recovers from panics in downstream handlers, logs the panic with
// the request's trace_id and stack trace, and returns a clean 500 response so a
// single handler panic never takes down the whole HTTP server.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// http.ErrAbortHandler is used intentionally to abort a connection;
				// re-panic so the server handles it as designed.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				slog.Error("http handler panic recovered",
					"error", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"trace_id", GetTraceID(r.Context()),
					"stack", string(debug.Stack()),
				)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"detail":"Internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
