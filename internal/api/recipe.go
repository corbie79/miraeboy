package api

import (
	"fmt"
	"net/http"
)

// GET /{context}/v2/conans/search?q=<query>
func (s *Server) handleRecipeSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.PathValue("context")
	query := r.URL.Query().Get("q")

	results, err := s.store.Search(ctx, query)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// GET /{context}/v2/conans/{name}/{version}/{username}/{channel}/revisions
func (s *Server) handleListRecipeRevisions(w http.ResponseWriter, r *http.Request) {
	ctx, name, version, username, channel := recipeParams(r)
	revs, err := s.store.GetRecipeRevisions(ctx, name, version, username, channel)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revs})
}

// GET /{context}/v2/conans/{name}/{version}/{username}/{channel}/revisions/latest
func (s *Server) handleLatestRecipeRevision(w http.ResponseWriter, r *http.Request) {
	ctx, name, version, username, channel := recipeParams(r)
	revs, err := s.store.GetRecipeRevisions(ctx, name, version, username, channel)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(revs) == 0 {
		jsonError(w, http.StatusNotFound, "no revisions found")
		return
	}
	writeJSON(w, http.StatusOK, revs[0])
}

// GET /{context}/v2/conans/{name}/{version}/{username}/{channel}/revisions/{rrev}/files
func (s *Server) handleListRecipeFiles(w http.ResponseWriter, r *http.Request) {
	ctx, name, version, username, channel := recipeParams(r)
	rrev := r.PathValue("rrev")

	if !s.store.RecipeRevisionExists(ctx, name, version, username, channel, rrev) {
		jsonError(w, http.StatusNotFound, "revision not found")
		return
	}
	files, err := s.store.ListRecipeFiles(ctx, name, version, username, channel, rrev)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

// GET /{context}/v2/conans/{name}/{version}/{username}/{channel}/revisions/{rrev}/files/{filename...}
func (s *Server) handleDownloadRecipeFile(w http.ResponseWriter, r *http.Request) {
	ctx, name, version, username, channel := recipeParams(r)
	rrev := r.PathValue("rrev")
	filename := r.PathValue("filename")

	rc, size, err := s.store.GetRecipeFile(ctx, name, version, username, channel, rrev, filename)
	if err != nil {
		jsonError(w, http.StatusNotFound, "file not found")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.WriteHeader(http.StatusOK)
	streamBody(w, rc)
}

// PUT /{context}/v2/conans/{name}/{version}/{username}/{channel}/revisions/{rrev}/files/{filename...}
func (s *Server) handleUploadRecipeFile(w http.ResponseWriter, r *http.Request) {
	ctx, name, version, username, channel := recipeParams(r)
	rrev := r.PathValue("rrev")
	filename := r.PathValue("filename")

	if err := s.store.PutRecipeFile(ctx, name, version, username, channel, rrev, filename, r.Body); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.AddRecipeRevision(ctx, name, version, username, channel, rrev); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// DELETE /{context}/v2/conans/{name}/{version}/{username}/{channel}/revisions/{rrev}
func (s *Server) handleDeleteRecipeRevision(w http.ResponseWriter, r *http.Request) {
	ctx, name, version, username, channel := recipeParams(r)
	rrev := r.PathValue("rrev")

	if !s.store.RecipeRevisionExists(ctx, name, version, username, channel, rrev) {
		jsonError(w, http.StatusNotFound, "revision not found")
		return
	}
	if err := s.store.DeleteRecipeRevision(ctx, name, version, username, channel, rrev); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func recipeParams(r *http.Request) (ctx, name, version, username, channel string) {
	return r.PathValue("context"), r.PathValue("name"), r.PathValue("version"),
		r.PathValue("username"), r.PathValue("channel")
}
