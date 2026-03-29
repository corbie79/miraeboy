package api

import (
	"net/http"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/storage"
)

// GET /api/conan/{repository}/v2/users/authenticate
// Client sends Basic Auth credentials; server returns a Bearer token.
// The token embeds the user's repository permissions so every subsequent request
// can be authorized without hitting storage again.
func (s *Server) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Conan2 Repository"`)
		jsonError(w, http.StatusUnauthorized, "basic auth required")
		return
	}

	// Check storage first (API-created users), then fall back to config.yaml.
	var isAdmin bool
	storedUser, err := s.store.FindUser(username, storage.HashPassword(password))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "auth error")
		return
	}
	if storedUser != nil {
		isAdmin = storedUser.Admin
	} else {
		cfgUser := s.cfg.FindUser(username, password)
		if cfgUser == nil {
			jsonError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		isAdmin = cfgUser.Admin
	}

	groups := make(map[string]auth.Permission)
	if isAdmin {
		groups["*"] = auth.PermOwner
	} else {
		repoPerms, err := s.store.GetUserRepoPermissions(username)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to load repository permissions")
			return
		}
		for repo, p := range repoPerms {
			groups[repo] = auth.Permission(p)
		}
	}

	token, err := auth.IssueToken(s.cfg.Auth.JWTSecret, username, isAdmin, groups)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// GET /api/conan/{repository}/v2/users/check_credentials
// Validates that the current Bearer token is still valid.
func (s *Server) handleCheckCredentials(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
