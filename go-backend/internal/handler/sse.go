package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/smartedu/training-eval-system/internal/crypto"
	"github.com/smartedu/training-eval-system/internal/sse"
)

// SSEHandler serves Server-Sent Events for real-time push to the frontend.
// Frontend: use EventSource('/api/sse/events?token=xxx') or fetch + ReadableStream.
// This replaces the Python backend's WebSocket /ws/{channel} endpoints.
type SSEHandler struct {
	broker    *sse.Broker
	jwtSecret string
}

// NewSSEHandler creates an SSE handler.
func NewSSEHandler(broker *sse.Broker, jwtSecret string) *SSEHandler {
	return &SSEHandler{broker: broker, jwtSecret: jwtSecret}
}

// Events handles the SSE connection.
// The frontend connects via:
//
//	const es = new EventSource('/api/sse/events?token=' + encodeURIComponent(token))
//	es.addEventListener('progress', (e) => { ... })
//	es.addEventListener('notification', (e) => { ... })
//	es.addEventListener('score_complete', (e) => { ... })
func (h *SSEHandler) Events(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query parameter token or Authorization header
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}

	if token == "" {
		Error(w, http.StatusUnauthorized, "Missing token")
		return
	}

	// Validate JWT to extract user ID
	claims, err := crypto.VerifyToken(h.jwtSecret, token)
	if err != nil {
		Error(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	// Register with broker
	client := h.broker.Subscribe(claims.Sub)
	defer h.broker.Unsubscribe(client)

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"user_id\":%d,\"ts\":%q}\n\n", claims.Sub, time.Now().Format(time.RFC3339))
	flusher.Flush()

	// Event loop: pump broker events to the SSE response writer
	for {
		select {
		case event, ok := <-client.Ch:
			if !ok {
				// Broker shutting down
				return
			}
			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
			if err != nil {
				slog.Debug("SSE write error (client disconnected)", "user_id", claims.Sub, "error", err.Error())
				return
			}
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			return

		case <-time.After(30 * time.Second):
			// Send keepalive comment to prevent proxy timeout
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
