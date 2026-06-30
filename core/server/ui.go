package server

import "net/http"

// registerUI mounts the fort-ui event/command + live-feed routes. Implemented
// in Phase 3 (AO-031/033/035); a no-op until UI deps (engine + store) exist.
func (s *Server) registerUI(mux *http.ServeMux) {
	if s.deps.Store == nil {
		return
	}
	// Phase 3 routes are registered here.
}
