package api

import (
	"net/http"

	"github.com/corbie79/miraeboy/internal/auth"
)

// GET /v2/users/authenticate
// Client sends Basic Auth credentials; server returns a Bearer token.
func (s *Server) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Conan2 Repository"`)
		jsonError(w, http.StatusUnauthorized, "basic auth required")
		return
	}

	user := s.cfg.FindUser(username, password)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.IssueToken(s.cfg.Auth.JWTSecret, user.Username, user.Admin)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// GET /v2/users/check_credentials
// Validates that the current Bearer token is still valid.
func (s *Server) handleCheckCredentials(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
