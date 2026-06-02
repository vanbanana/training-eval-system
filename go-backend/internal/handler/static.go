package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticHandler serves frontend static files with SPA fallback.
type StaticHandler struct {
	distDir string
}

// NewStaticHandler creates a static file handler.
func NewStaticHandler(distDir string) *StaticHandler {
	return &StaticHandler{distDir: distDir}
}

// ServeHTTP serves static files or falls back to index.html for SPA routing.
func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only serve GET/HEAD
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	// Don't serve API routes
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
		http.NotFound(w, r)
		return
	}

	// Try to serve the exact file
	path := filepath.Join(h.distDir, filepath.Clean(r.URL.Path))

	// Check if file exists
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}

	// SPA fallback: serve index.html
	indexPath := filepath.Join(h.distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		// dist/ doesn't exist — graceful degradation
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, indexPath)
}
