package api

import (
	"net/http"

	"github.com/corbie79/miraeboy/internal/auth"
)

// POST /api/auth/login
// Web UI용 독립 로그인 엔드포인트. JSON body로 credentials를 받아 JWT를 반환.
func (s *Server) handleWebLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user := s.cfg.FindUser(req.Username, req.Password)
	if user == nil {
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	groups := make(map[string]auth.Permission)
	if user.Admin {
		groups["*"] = auth.PermOwner
	} else {
		repoPerms, err := s.store.GetUserRepoPermissions(user.Username)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to load permissions")
			return
		}
		for repo, p := range repoPerms {
			groups[repo] = auth.Permission(p)
		}
	}

	token, err := auth.IssueToken(s.cfg.Auth.JWTSecret, user.Username, user.Admin, groups)
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
