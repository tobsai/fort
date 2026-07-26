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
	"github.com/tobsai/fort/core/requestid"
	"github.com/tobsai/fort/core/store"
)

// RequestEvent is the payload-free ingress trace emitted after one HTTP
// request. Path excludes the query string.
type RequestEvent struct {
	ID       string
	Method   string
	Path     string
	Status   int
	Duration time.Duration
}

type RequestObserver func(RequestEvent)

// Deps are the server's collaborators. All are optional for the bare /health
// server; richer routes require engine + store.
type Deps struct {
	Config          config.Config
	Engine          *engine.Engine
	Store           *store.Store
	Logger          *slog.Logger
	RequestObserver RequestObserver
	// Mount optionally adds extra routes (the fort-ui module, wired in by
	// cmd/fort). core never imports ui — the closure is provided from outside,
	// preserving the core !-> ui seam.
	Mount func(*http.ServeMux)
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
	if s.deps.Mount != nil {
		s.deps.Mount(mux) // fort-ui routes, injected by cmd/fort
	}
	observer := s.deps.RequestObserver
	if observer == nil {
		observer = func(event RequestEvent) {
			s.log.Info("fort request", "request_id", event.ID, "method", event.Method,
				"path", event.Path, "status", event.Status, "duration", event.Duration)
		}
	}
	return ObserveRequests(mux, observer)
}

// ObserveRequests adds one canonical correlation ID and emits a payload-free
// completion event. It is shared by the local listener and the relay mux.
func ObserveRequests(next http.Handler, observer RequestObserver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestid.Header)
		if !requestid.Valid(id) {
			id = requestid.New()
		}
		w.Header().Set(requestid.Header, id)
		wrapped := &requestWriter{ResponseWriter: w}
		started := time.Now()
		next.ServeHTTP(wrapped, r.WithContext(requestid.With(r.Context(), id)))
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		if observer != nil {
			observer(RequestEvent{
				ID: id, Method: r.Method, Path: r.URL.Path,
				Status: status, Duration: time.Since(started),
			})
		}
	})
}

type requestWriter struct {
	http.ResponseWriter
	status int
}

func (w *requestWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *requestWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

func (w *requestWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *requestWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

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
