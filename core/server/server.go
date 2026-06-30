// Package server is fort-core's local HTTP/WS API (backlog AO-011): /health,
// graceful shutdown, and the event/command surface the fort-ui module consumes
// (the live-feed and command routes are added in Phase 3 on top of this).
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/store"
)

// Deps are the server's collaborators. All are optional for the bare /health
// server; richer routes require engine + store.
type Deps struct {
	Config config.Config
	Engine *engine.Engine
	Store  *store.Store
	Logger *slog.Logger
}

// Server serves the fort-core API.
type Server struct {
	deps Deps
	log  *slog.Logger
}

// New builds a server.
func New(d Deps) *Server {
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Server{deps: d, log: log}
}

// Handler returns the HTTP handler (exposed for tests).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	s.registerUI(mux) // Phase 3 routes (no-op if UI deps absent)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "fort-core",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// Run starts the server and blocks until ctx is canceled, then shuts down
// gracefully.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.deps.Config.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("fort-core listening", "addr", s.deps.Config.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.log.Info("fort-core shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
