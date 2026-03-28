package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/corbie79/miraeboy/internal/storage"
)

var validGroupName = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$|^[a-z0-9]$`)

var reservedGroupNames = map[string]bool{
	"api":    true,
	"ping":   true,
	"_":      true,
	"groups": true,
}

// ─── request / response types ─────────────────────────────────────────────────

type groupCreateReq struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Owner           string `json:"owner"`
	ConanUser       string `json:"conan_user"`
	ConanChannel    string `json:"conan_channel"`
	AnonymousAccess string `json:"anonymous_access"`
}

type groupUpdateReq struct {
	Description     *string `json:"description"`
	ConanUser       *string `json:"conan_user"`
	ConanChannel    *string `json:"conan_channel"`
	AnonymousAccess *string `json:"anonymous_access"`
}

type groupResp struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Owner           string `json:"owner"`
	ConanUser       string `json:"conan_user"`
	ConanChannel    string `json:"conan_channel"`
	AnonymousAccess string `json:"anonymous_access"`
	Source          string `json:"source"`
	MemberCount     int    `json:"member_count"`
}

type memberReq struct {
	Username   string `json:"username"`
	Permission string `json:"permission"` // "read", "write", "delete", "owner"
}

func toGroupResp(g storage.GroupRecord) groupResp {
	return groupResp{
		Name:            g.Name,
		Description:     g.Description,
		Owner:           g.Owner,
		ConanUser:       g.ConanUser,
		ConanChannel:    g.ConanChannel,
		AnonymousAccess: g.AnonymousAccess,
		Source:          g.Source,
		MemberCount:     len(g.Members),
	}
}

// ─── group CRUD ───────────────────────────────────────────────────────────────

// POST /api/groups  (admin only)
func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req groupCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validGroupName.MatchString(req.Name) {
		jsonError(w, http.StatusBadRequest, "invalid group name: use lowercase letters, digits, and hyphens")
		return
	}
	if reservedGroupNames[req.Name] {
		jsonError(w, http.StatusBadRequest, "group name is reserved: "+req.Name)
		return
	}
	if req.Owner == "" {
		jsonError(w, http.StatusBadRequest, "owner is required")
		return
	}
	if req.AnonymousAccess == "" {
		req.AnonymousAccess = "none"
	}
	if s.store.GroupExists(req.Name) {
		jsonError(w, http.StatusConflict, "group already exists: "+req.Name)
		return
	}

	g := storage.GroupRecord{
		Name:            req.Name,
		Description:     req.Description,
		Owner:           req.Owner,
		ConanUser:       req.ConanUser,
		ConanChannel:    req.ConanChannel,
		AnonymousAccess: req.AnonymousAccess,
		Source:          "api",
		Members:         []storage.GroupMember{},
	}
	if err := s.store.SaveGroup(g); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toGroupResp(g))
}

// GET /api/groups  (admin only — returns all groups)
func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.store.ListGroups()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]groupResp, len(groups))
	for i, g := range groups {
		resp[i] = toGroupResp(g)
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": resp})
}

// GET /api/groups/{group}  (admin only)
func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("group")
	g, err := s.store.GetGroup(name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if g == nil {
		jsonError(w, http.StatusNotFound, "group not found: "+name)
		return
	}
	writeJSON(w, http.StatusOK, toGroupResp(*g))
}

// PATCH /api/groups/{group}  (owner or admin)
// Only description, conan_user, conan_channel, anonymous_access can be changed.
// Owner transfer is not supported via this endpoint.
func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("group")
	g, err := s.store.GetGroup(name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if g == nil {
		jsonError(w, http.StatusNotFound, "group not found: "+name)
		return
	}

	var req groupUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Description != nil {
		g.Description = *req.Description
	}
	if req.ConanUser != nil {
		g.ConanUser = *req.ConanUser
	}
	if req.ConanChannel != nil {
		g.ConanChannel = *req.ConanChannel
	}
	if req.AnonymousAccess != nil {
		g.AnonymousAccess = *req.AnonymousAccess
	}

	if err := s.store.SaveGroup(*g); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toGroupResp(*g))
}

// DELETE /api/groups/{group}?force=true  (admin only)
func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("group")
	force := r.URL.Query().Get("force") == "true"

	if !s.store.GroupExists(name) {
		jsonError(w, http.StatusNotFound, "group not found: "+name)
		return
	}
	if !force {
		results, _ := s.store.Search(name, "*")
		if len(results) > 0 {
			jsonError(w, http.StatusConflict, "group is not empty; use ?force=true to delete with all packages")
			return
		}
	}
	if err := s.store.DeleteGroup(name); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── member management ────────────────────────────────────────────────────────

var validPermissions = map[string]bool{
	"read": true, "write": true, "delete": true, "owner": true,
}

// POST /api/groups/{group}/members  (owner or admin)
func (s *Server) handleInviteMember(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("group")
	g, err := s.store.GetGroup(name)
	if err != nil || g == nil {
		jsonError(w, http.StatusNotFound, "group not found: "+name)
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
	if req.Username == g.Owner && req.Permission != "owner" {
		jsonError(w, http.StatusBadRequest, "cannot change owner's permission; transfer ownership instead")
		return
	}

	// Update or add
	updated := false
	for i, m := range g.Members {
		if m.Username == req.Username {
			g.Members[i].Permission = req.Permission
			updated = true
			break
		}
	}
	if !updated {
		g.Members = append(g.Members, storage.GroupMember{
			Username:   req.Username,
			Permission: req.Permission,
		})
	}

	if err := s.store.SaveGroup(*g); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":   req.Username,
		"permission": req.Permission,
	})
}

// GET /api/groups/{group}/members  (owner or admin)
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("group")
	g, err := s.store.GetGroup(name)
	if err != nil || g == nil {
		jsonError(w, http.StatusNotFound, "group not found: "+name)
		return
	}

	type memberResp struct {
		Username   string `json:"username"`
		Permission string `json:"permission"`
		IsOwner    bool   `json:"is_owner"`
	}

	members := make([]memberResp, 0, len(g.Members)+1)
	// Owner is always listed first
	members = append(members, memberResp{
		Username:   g.Owner,
		Permission: "owner",
		IsOwner:    true,
	})
	for _, m := range g.Members {
		if m.Username == g.Owner {
			continue // skip duplicate if owner is also in Members list
		}
		members = append(members, memberResp{
			Username:   m.Username,
			Permission: m.Permission,
			IsOwner:    false,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

// PUT /api/groups/{group}/members/{username}  (owner or admin)
func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	groupName := r.PathValue("group")
	targetUser := r.PathValue("username")

	g, err := s.store.GetGroup(groupName)
	if err != nil || g == nil {
		jsonError(w, http.StatusNotFound, "group not found: "+groupName)
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
	if targetUser == g.Owner {
		jsonError(w, http.StatusBadRequest, "cannot change owner's permission")
		return
	}

	found := false
	for i, m := range g.Members {
		if m.Username == targetUser {
			g.Members[i].Permission = req.Permission
			found = true
			break
		}
	}
	if !found {
		jsonError(w, http.StatusNotFound, "member not found: "+targetUser)
		return
	}

	if err := s.store.SaveGroup(*g); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":   targetUser,
		"permission": req.Permission,
	})
}

// DELETE /api/groups/{group}/members/{username}  (owner or admin)
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	groupName := r.PathValue("group")
	targetUser := r.PathValue("username")

	g, err := s.store.GetGroup(groupName)
	if err != nil || g == nil {
		jsonError(w, http.StatusNotFound, "group not found: "+groupName)
		return
	}
	if targetUser == g.Owner {
		jsonError(w, http.StatusBadRequest, "cannot remove the group owner")
		return
	}

	filtered := g.Members[:0]
	found := false
	for _, m := range g.Members {
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
	g.Members = filtered

	if err := s.store.SaveGroup(*g); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
