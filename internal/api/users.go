package api

import (
	"net/http"

	"github.com/corbie79/miraeboy/internal/auth"
)

// GET /{context}/v2/users/authenticate
// Client sends Basic Auth credentials; server returns a Bearer token.
// The token embeds the user's context permissions so every subsequent request
// can be authorized without hitting the config again.
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

	contextMap := s.cfg.BuildUserContextMap(user)
	token, err := auth.IssueToken(s.cfg.Auth.JWTSecret, user.Username, user.Admin, contextMap)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// GET /{context}/v2/users/check_credentials
// Validates that the current Bearer token is still valid.
func (s *Server) handleCheckCredentials(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
