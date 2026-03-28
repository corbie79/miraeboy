package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/corbie79/miraeboy/internal/storage"
)

var validRepoName = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$|^[a-z0-9]$`)

var reservedRepoNames = map[string]bool{
	"api":   true,
	"ping":  true,
	"_":     true,
	"repos": true,
	"conan": true,
}

// ─── request / response types ─────────────────────────────────────────────────

type repoCreateReq struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Owner             string   `json:"owner"`
	AllowedNamespaces []string `json:"allowed_namespaces"`
	AllowedChannels   []string `json:"allowed_channels"`
	AnonymousAccess   string   `json:"anonymous_access"`
}

type repoUpdateReq struct {
	Description       *string  `json:"description"`
	AllowedNamespaces []string `json:"allowed_namespaces"`
	AllowedChannels   []string `json:"allowed_channels"`
	AnonymousAccess   *string  `json:"anonymous_access"`
}

type repoResp struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Owner             string   `json:"owner"`
	AllowedNamespaces []string `json:"allowed_namespaces"`
	AllowedChannels   []string `json:"allowed_channels"`
	AnonymousAccess   string   `json:"anonymous_access"`
	Source            string   `json:"source"`
	MemberCount       int      `json:"member_count"`
}

type memberReq struct {
	Username   string `json:"username"`
	Permission string `json:"permission"` // "read", "write", "delete", "owner"
}

func toRepoResp(r storage.RepoRecord) repoResp {
	ns := r.AllowedNamespaces
	if ns == nil {
		ns = []string{}
	}
	ch := r.AllowedChannels
	if ch == nil {
		ch = []string{}
	}
	return repoResp{
		Name:              r.Name,
		Description:       r.Description,
		Owner:             r.Owner,
		AllowedNamespaces: ns,
		AllowedChannels:   ch,
		AnonymousAccess:   r.AnonymousAccess,
		Source:            r.Source,
		MemberCount:       len(r.Members),
	}
}

// ─── repository CRUD ──────────────────────────────────────────────────────────

// POST /api/repos  (admin only)
func (s *Server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	var req repoCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validRepoName.MatchString(req.Name) {
		jsonError(w, http.StatusBadRequest, "invalid repository name: use lowercase letters, digits, and hyphens")
		return
	}
	if reservedRepoNames[req.Name] {
		jsonError(w, http.StatusBadRequest, "repository name is reserved: "+req.Name)
		return
	}
	if req.Owner == "" {
		jsonError(w, http.StatusBadRequest, "owner is required")
		return
	}
	if req.AnonymousAccess == "" {
		req.AnonymousAccess = "none"
	}
	if s.store.RepoExists(req.Name) {
		jsonError(w, http.StatusConflict, "repository already exists: "+req.Name)
		return
	}

	rec := storage.RepoRecord{
		Name:              req.Name,
		Description:       req.Description,
		Owner:             req.Owner,
		AllowedNamespaces: req.AllowedNamespaces,
		AllowedChannels:   req.AllowedChannels,
		AnonymousAccess:   req.AnonymousAccess,
		Source:            "api",
		Members:           []storage.RepoMember{},
	}
	if err := s.store.SaveRepo(rec); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toRepoResp(rec))
}

// GET /api/repos  (admin only)
func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := s.store.ListRepos()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]repoResp, len(repos))
	for i, repo := range repos {
		resp[i] = toRepoResp(repo)
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": resp})
}

// GET /api/repos/{repository}  (admin only)
func (s *Server) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("repository")
	repo, err := s.store.GetRepo(name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if repo == nil {
		jsonError(w, http.StatusNotFound, "repository not found: "+name)
		return
	}
	writeJSON(w, http.StatusOK, toRepoResp(*repo))
}

// PATCH /api/repos/{repository}  (owner or admin)
// Updates description, allowed_namespaces, allowed_channels, anonymous_access.
func (s *Server) handleUpdateRepo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("repository")
	repo, err := s.store.GetRepo(name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if repo == nil {
		jsonError(w, http.StatusNotFound, "repository not found: "+name)
		return
	}

	var req repoUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Description != nil {
		repo.Description = *req.Description
	}
	if req.AllowedNamespaces != nil {
		repo.AllowedNamespaces = req.AllowedNamespaces
	}
	if req.AllowedChannels != nil {
		repo.AllowedChannels = req.AllowedChannels
	}
	if req.AnonymousAccess != nil {
		repo.AnonymousAccess = *req.AnonymousAccess
	}

	if err := s.store.SaveRepo(*repo); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRepoResp(*repo))
}

// DELETE /api/repos/{repository}?force=true  (admin only)
func (s *Server) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("repository")
	force := r.URL.Query().Get("force") == "true"

	if !s.store.RepoExists(name) {
		jsonError(w, http.StatusNotFound, "repository not found: "+name)
		return
	}
	if !force {
		results, _ := s.store.Search(name, "*")
		if len(results) > 0 {
			jsonError(w, http.StatusConflict, "repository is not empty; use ?force=true to delete with all packages")
			return
		}
	}
	if err := s.store.DeleteRepo(name); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── member management ────────────────────────────────────────────────────────

var validPermissions = map[string]bool{
	"read": true, "write": true, "delete": true, "owner": true,
}

// POST /api/repos/{repository}/members  (owner or admin)
func (s *Server) handleInviteMember(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("repository")
	repo, err := s.store.GetRepo(name)
	if err != nil || repo == nil {
		jsonError(w, http.StatusNotFound, "repository not found: "+name)
		return
	}

	var req memberReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" {
		jsonError(w, http.StatusBadRequest, "username is required")
		return
	}
	if !validPermissions[req.Permission] {
		jsonError(w, http.StatusBadRequest, "permission must be one of: read, write, delete, owner")
		return
	}
	if req.Username == repo.Owner && req.Permission != "owner" {
		jsonError(w, http.StatusBadRequest, "cannot change owner's permission; transfer ownership instead")
		return
	}

	updated := false
	for i, m := range repo.Members {
		if m.Username == req.Username {
			repo.Members[i].Permission = req.Permission
			updated = true
			break
		}
	}
	if !updated {
		repo.Members = append(repo.Members, storage.RepoMember{
			Username:   req.Username,
			Permission: req.Permission,
		})
	}

	if err := s.store.SaveRepo(*repo); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":   req.Username,
		"permission": req.Permission,
	})
}

// GET /api/repos/{repository}/members  (owner or admin)
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("repository")
	repo, err := s.store.GetRepo(name)
	if err != nil || repo == nil {
		jsonError(w, http.StatusNotFound, "repository not found: "+name)
		return
	}

	type memberResp struct {
		Username   string `json:"username"`
		Permission string `json:"permission"`
		IsOwner    bool   `json:"is_owner"`
	}

	members := make([]memberResp, 0, len(repo.Members)+1)
	members = append(members, memberResp{
		Username:   repo.Owner,
		Permission: "owner",
		IsOwner:    true,
	})
	for _, m := range repo.Members {
		if m.Username == repo.Owner {
			continue
		}
		members = append(members, memberResp{
			Username:   m.Username,
			Permission: m.Permission,
			IsOwner:    false,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

// PUT /api/repos/{repository}/members/{username}  (owner or admin)
func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("repository")
	targetUser := r.PathValue("username")

	repo, err := s.store.GetRepo(repoName)
	if err != nil || repo == nil {
		jsonError(w, http.StatusNotFound, "repository not found: "+repoName)
		return
	}

	var req struct {
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validPermissions[req.Permission] {
		jsonError(w, http.StatusBadRequest, "permission must be one of: read, write, delete, owner")
		return
	}
	if targetUser == repo.Owner {
		jsonError(w, http.StatusBadRequest, "cannot change owner's permission")
		return
	}

	found := false
	for i, m := range repo.Members {
		if m.Username == targetUser {
			repo.Members[i].Permission = req.Permission
			found = true
			break
		}
	}
	if !found {
		jsonError(w, http.StatusNotFound, "member not found: "+targetUser)
		return
	}

	if err := s.store.SaveRepo(*repo); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":   targetUser,
		"permission": req.Permission,
	})
}

// DELETE /api/repos/{repository}/members/{username}  (owner or admin)
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("repository")
	targetUser := r.PathValue("username")

	repo, err := s.store.GetRepo(repoName)
	if err != nil || repo == nil {
		jsonError(w, http.StatusNotFound, "repository not found: "+repoName)
		return
	}
	if targetUser == repo.Owner {
		jsonError(w, http.StatusBadRequest, "cannot remove the repository owner")
		return
	}

	filtered := repo.Members[:0]
	found := false
	for _, m := range repo.Members {
		if m.Username == targetUser {
			found = true
			continue
		}
		filtered = append(filtered, m)
	}
	if !found {
		jsonError(w, http.StatusNotFound, "member not found: "+targetUser)
		return
	}
	repo.Members = filtered

	if err := s.store.SaveRepo(*repo); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
