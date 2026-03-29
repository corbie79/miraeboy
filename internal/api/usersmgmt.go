package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/corbie79/miraeboy/internal/storage"
)

var validUsername = regexp.MustCompile(`^[a-z0-9][a-z0-9_\-]*$`)

type userResp struct {
	Username  string    `json:"username"`
	Admin     bool      `json:"admin"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResp(u storage.UserRecord) userResp {
	return userResp{
		Username:  u.Username,
		Admin:     u.Admin,
		Source:    u.Source,
		CreatedAt: u.CreatedAt,
	}
}

// GET /api/users  (admin only)
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]userResp, len(users))
	for i, u := range users {
		resp[i] = toUserResp(u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": resp})
}

// GET /api/users/{username}  (admin only)
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	u, err := s.store.GetUser(username)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if u == nil {
		jsonError(w, http.StatusNotFound, "user not found: "+username)
		return
	}
	writeJSON(w, http.StatusOK, toUserResp(*u))
}

// POST /api/users  (admin only)
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validUsername.MatchString(req.Username) {
		jsonError(w, http.StatusBadRequest, "invalid username: use lowercase letters, digits, hyphens, underscores")
		return
	}
	if len(req.Password) < 6 {
		jsonError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}
	if s.store.UserExists(req.Username) {
		jsonError(w, http.StatusConflict, "user already exists: "+req.Username)
		return
	}

	u := storage.UserRecord{
		Username:     req.Username,
		PasswordHash: storage.HashPassword(req.Password),
		Admin:        req.Admin,
		CreatedAt:    time.Now().UTC(),
		Source:       "api",
	}
	if err := s.store.SaveUser(u); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toUserResp(u))
}

// PATCH /api/users/{username}  (admin only)
// Updates password and/or admin flag.
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	u, err := s.store.GetUser(username)
	if err != nil || u == nil {
		jsonError(w, http.StatusNotFound, "user not found: "+username)
		return
	}

	var req struct {
		Password *string `json:"password"`
		Admin    *bool   `json:"admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Password != nil {
		if len(*req.Password) < 6 {
			jsonError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		u.PasswordHash = storage.HashPassword(*req.Password)
	}
	if req.Admin != nil {
		u.Admin = *req.Admin
	}

	if err := s.store.SaveUser(*u); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toUserResp(*u))
}

// DELETE /api/users/{username}  (admin only)
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !s.store.UserExists(username) {
		jsonError(w, http.StatusNotFound, "user not found: "+username)
		return
	}
	if err := s.store.DeleteUser(username); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
