package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

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

// pageParams parses ?page=N&limit=N query parameters.
// Returns page (1-based) and limit with sensible defaults and caps.
func pageParams(r *http.Request) (page, limit int) {
	page = 1
	limit = 50
	if v := r.URL.Query().Get("page"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}
	return
}

// ─── request / response types ─────────────────────────────────────────────────

type gitConfigReq struct {
	URL    string `json:"url"`    // HTTPS clone URL; empty string disables git sync
	Branch string `json:"branch"` // default: "main"
	Token  string `json:"token"`  // PAT or equivalent (write-only; omitted from responses)
}

type repoCreateReq struct {
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Owner             string         `json:"owner"`
	AllowedNamespaces []string       `json:"allowed_namespaces"`
	AllowedChannels   []string       `json:"allowed_channels"`
	AnonymousAccess   string         `json:"anonymous_access"`
	Git               *gitConfigReq  `json:"git,omitempty"`
}

type repoUpdateReq struct {
	Description       *string        `json:"description"`
	AllowedNamespaces []string       `json:"allowed_namespaces"`
	AllowedChannels   []string       `json:"allowed_channels"`
	AnonymousAccess   *string        `json:"anonymous_access"`
	Git               *gitConfigReq  `json:"git,omitempty"` // set to {} with empty url to disable
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
	GitURL            string   `json:"git_url,omitempty"`    // configured git remote (token omitted)
	GitBranch         string   `json:"git_branch,omitempty"` // configured git branch
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
	resp := repoResp{
		Name:              r.Name,
		Description:       r.Description,
		Owner:             r.Owner,
		AllowedNamespaces: ns,
		AllowedChannels:   ch,
		AnonymousAccess:   r.AnonymousAccess,
		Source:            r.Source,
		MemberCount:       len(r.Members),
	}
	if r.Git != nil && r.Git.URL != "" {
		resp.GitURL = r.Git.URL
		resp.GitBranch = r.Git.Branch
	}
	return resp
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
		Git:               gitConfigFromReq(req.Git),
	}
	if err := s.store.SaveRepo(rec); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toRepoResp(rec))
}

// GET /api/repos
// Admin: returns all repositories.
// Non-admin: returns only repositories where the user is a member or owner.
func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())

	repos, err := s.store.ListRepos()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var filtered []repoResp
	for _, repo := range repos {
		if claims.Admin || isRepoMember(repo, claims.Username) {
			filtered = append(filtered, toRepoResp(repo))
		}
	}
	if filtered == nil {
		filtered = []repoResp{}
	}
	page, limit := pageParams(r)
	total := len(filtered)
	offset := (page - 1) * limit
	if offset < len(filtered) {
		end := offset + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[offset:end]
	} else {
		filtered = []repoResp{}
	}
	pages := (total + limit - 1) / limit
	writeJSON(w, http.StatusOK, map[string]any{
		"repositories": filtered,
		"total":        total,
		"page":         page,
		"pages":        pages,
	})
}

// GET /api/repos/{repository}
// Admin: always returns the repo. Non-admin: only if member or owner.
func (s *Server) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromContext(r.Context())
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
	if !claims.Admin && !isRepoMember(*repo, claims.Username) {
		jsonError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, toRepoResp(*repo))
}

// isRepoMember returns true if username is the owner or an explicit member.
func isRepoMember(repo storage.RepoRecord, username string) bool {
	if repo.Owner == username {
		return true
	}
	for _, m := range repo.Members {
		if m.Username == username {
			return true
		}
	}
	return false
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
	if req.Git != nil {
		repo.Git = gitConfigFromReq(req.Git)
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

// gitConfigFromReq converts a gitConfigReq to a storage.GitSyncConfig.
// Returns nil when req is nil or URL is empty (disables git sync).
func gitConfigFromReq(req *gitConfigReq) *storage.GitSyncConfig {
	if req == nil || req.URL == "" {
		return nil
	}
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	return &storage.GitSyncConfig{
		URL:    req.URL,
		Branch: branch,
		Token:  req.Token,
	}
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

// POST /api/repos/{repository}/gc?keep=5&dry_run=false  (owner or admin)
func (s *Server) handleRepoGC(w http.ResponseWriter, r *http.Request) {
	repoName := r.PathValue("repository")

	keep := 5
	if v := r.URL.Query().Get("keep"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			keep = n
		}
	}
	dryRun := r.URL.Query().Get("dry_run") == "true"

	result, err := s.store.GCRepo(repoName, keep, dryRun)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"dry_run":           dryRun,
		"keep":              keep,
		"revisions_deleted": result.RevisionsDeleted,
		"packages_deleted":  result.PackagesDeleted,
	})
}
