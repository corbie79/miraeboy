package api

import "net/http"

// GET /ping
// Conan clients use this endpoint to detect a v2-compatible server.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": "v2"})
}
