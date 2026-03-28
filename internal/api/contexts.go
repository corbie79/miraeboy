package api

import (
	"encoding/json"
	"net/http"
	"regexp"
)

// validContextName allows lowercase letters, digits, and hyphens (no leading/trailing hyphens).
var validContextName = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*[a-z0-9]$|^[a-z0-9]$`)

// reserved names that cannot be used as context names.
var reservedNames = map[string]bool{
	"api":      true,
	"users":    true,
	"contexts": true,
	"ping":     true,
	"_":        true,
}

type contextRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	AnonymousAccess string `json:"anonymous_access"` // "read", "readwrite", "none"
}

type contextResponse struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	AnonymousAccess string `json:"anonymous_access"`
	Source          string `json:"source"` // "config" or "api"
}

// POST /api/contexts
func (s *Server) handleCreateContext(w http.ResponseWriter, r *http.Request) {
	var req contextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validContextName.MatchString(req.Name) {
		jsonError(w, http.StatusBadRequest, "invalid context name: use lowercase letters, digits, and hyphens")
		return
	}
	if reservedNames[req.Name] {
		jsonError(w, http.StatusBadRequest, "context name is reserved: "+req.Name)
		return
	}

	// Check if already exists in config
	if s.cfg.FindContext(req.Name) != nil {
		jsonError(w, http.StatusConflict, "context already exists: "+req.Name)
		return
	}

	if req.AnonymousAccess == "" {
		req.AnonymousAccess = "none"
	}

	if err := s.store.AddDynamicContext(req.Name, req.Description, req.AnonymousAccess); err != nil {
		if isAlreadyExists(err) {
			jsonError(w, http.StatusConflict, "context already exists: "+req.Name)
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, contextResponse{
		Name:            req.Name,
		Description:     req.Description,
		AnonymousAccess: req.AnonymousAccess,
		Source:          "api",
	})
}

// GET /api/contexts
func (s *Server) handleListContexts(w http.ResponseWriter, r *http.Request) {
	var list []contextResponse

	// Static contexts from config.yaml
	for _, c := range s.cfg.Contexts {
		list = append(list, contextResponse{
			Name:            c.Name,
			Description:     c.Description,
			AnonymousAccess: c.AnonymousAccess,
			Source:          "config",
		})
	}

	// Dynamic contexts from storage
	recs, err := s.store.ListDynamicContexts()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, r := range recs {
		// Deduplicate: skip if already listed from config
		if s.cfg.FindContext(r.Name) != nil {
			continue
		}
		list = append(list, contextResponse{
			Name:            r.Name,
			Description:     r.Description,
			AnonymousAccess: r.AnonymousAccess,
			Source:          "api",
		})
	}

	if list == nil {
		list = []contextResponse{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contexts": list})
}

// GET /api/contexts/{context}
func (s *Server) handleGetContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("context")

	// Check config first
	if def := s.cfg.FindContext(name); def != nil {
		writeJSON(w, http.StatusOK, contextResponse{
			Name:            def.Name,
			Description:     def.Description,
			AnonymousAccess: def.AnonymousAccess,
			Source:          "config",
		})
		return
	}

	// Check dynamic
	recs, err := s.store.ListDynamicContexts()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, rec := range recs {
		if rec.Name == name {
			writeJSON(w, http.StatusOK, contextResponse{
				Name:            rec.Name,
				Description:     rec.Description,
				AnonymousAccess: rec.AnonymousAccess,
				Source:          "api",
			})
			return
		}
	}

	jsonError(w, http.StatusNotFound, "context not found: "+name)
}

// DELETE /api/contexts/{context}
// Only API-created contexts can be deleted (config contexts are read-only).
// Use ?force=true to delete even if the context contains packages.
func (s *Server) handleDeleteContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("context")
	force := r.URL.Query().Get("force") == "true"

	// Config-defined contexts cannot be deleted via API
	if s.cfg.FindContext(name) != nil {
		jsonError(w, http.StatusForbidden, "cannot delete a config-defined context; remove it from config.yaml")
		return
	}

	// Verify it exists in the dynamic registry
	recs, err := s.store.ListDynamicContexts()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	found := false
	for _, rec := range recs {
		if rec.Name == name {
			found = true
			break
		}
	}
	if !found {
		jsonError(w, http.StatusNotFound, "context not found: "+name)
		return
	}

	// Without force=true, refuse to delete non-empty contexts
	if !force {
		results, _ := s.store.Search(name, "*")
		if len(results) > 0 {
			jsonError(w, http.StatusConflict,
				"context is not empty; use ?force=true to delete with all packages")
			return
		}
	}

	if err := s.store.DeleteDynamicContext(name); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return len(err.Error()) > 0 && containsStr(err.Error(), "already exists")
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}

