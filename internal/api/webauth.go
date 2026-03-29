package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/storage"
)

// POST /api/auth/login
func (s *Server) handleWebLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.store.FindUser(req.Username, storage.HashPassword(req.Password))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "auth error")
		return
	}
	// fallback: config.yaml users not yet seeded
	if user == nil {
		cfgUser := s.cfg.FindUser(req.Username, req.Password)
		if cfgUser == nil {
			jsonError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		user = &storage.UserRecord{Username: cfgUser.Username, Admin: cfgUser.Admin}
	}

	token, err := s.issueTokenForUser(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"username": user.Username,
		"admin":    user.Admin,
	})
}

// POST /api/auth/refresh — reissues a fresh token from an existing valid token.
// Useful to extend sessions before the 24 h TTL expires.
func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if header == "" {
		jsonError(w, http.StatusUnauthorized, "authorization required")
		return
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		jsonError(w, http.StatusUnauthorized, "invalid authorization header")
		return
	}

	// Allow refresh up to 7 days after issue even if expired by normal TTL.
	claims, err := auth.ValidateTokenWithLeeway(s.cfg.Auth.JWTSecret, parts[1], 7*24*time.Hour)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Re-load user to pick up any permission changes since the old token was issued.
	user, err := s.store.GetUser(claims.Username)
	if err != nil || user == nil {
		// Fallback: reuse claims from the old token.
		newToken, err := auth.IssueToken(s.cfg.Auth.JWTSecret, claims.Username, claims.Admin, claims.Groups)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "token generation failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token":    newToken,
			"username": claims.Username,
			"admin":    claims.Admin,
		})
		return
	}

	newToken, err := s.issueTokenForUser(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":    newToken,
		"username": user.Username,
		"admin":    user.Admin,
	})
}

// issueTokenForUser builds the groups map and issues a JWT for the given user.
func (s *Server) issueTokenForUser(user *storage.UserRecord) (string, error) {
	groups := make(map[string]auth.Permission)
	if user.Admin {
		groups["*"] = auth.PermOwner
	} else {
		repoPerms, err := s.store.GetUserRepoPermissions(user.Username)
		if err != nil {
			return "", err
		}
		for repo, p := range repoPerms {
			groups[repo] = auth.Permission(p)
		}
	}
	return auth.IssueToken(s.cfg.Auth.JWTSecret, user.Username, user.Admin, groups)
}

