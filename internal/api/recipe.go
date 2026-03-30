package api

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/corbie79/miraeboy/internal/gitops"
	"github.com/corbie79/miraeboy/internal/metrics"
	"github.com/corbie79/miraeboy/internal/storage"
)

// GET /api/conan/{repository}/v2/conans/search?q=<query>
func (s *Server) handleRecipeSearch(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	query := r.URL.Query().Get("q")

	results, err := s.store.Search(repo, query)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []string{}
	}
	page, limit := pageParams(r)
	total := len(results)
	offset := (page - 1) * limit
	if offset < len(results) {
		end := offset + limit
		if end > len(results) {
			end = len(results)
		}
		results = results[offset:end]
	} else {
		results = []string{}
	}
	pages := (total + limit - 1) / limit
	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"total":   total,
		"page":    page,
		"pages":   pages,
	})
}

// GET /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions
func (s *Server) handleListRecipeRevisions(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel := recipeParams(r)
	revs, err := s.store.GetRecipeRevisions(repo, name, version, namespace, channel)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revs})
}

// GET /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/latest
func (s *Server) handleLatestRecipeRevision(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel := recipeParams(r)
	revs, err := s.store.GetRecipeRevisions(repo, name, version, namespace, channel)
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

// GET /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/files
func (s *Server) handleListRecipeFiles(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel := recipeParams(r)
	rrev := r.PathValue("rrev")

	if !s.store.RecipeRevisionExists(repo, name, version, namespace, channel, rrev) {
		jsonError(w, http.StatusNotFound, "revision not found")
		return
	}
	files, err := s.store.ListRecipeFiles(repo, name, version, namespace, channel, rrev)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

// GET /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/files/{filename...}
func (s *Server) handleDownloadRecipeFile(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel := recipeParams(r)
	rrev := r.PathValue("rrev")
	filename := r.PathValue("filename")

	rc, size, err := s.store.GetRecipeFile(repo, name, version, namespace, channel, rrev, filename)
	if err != nil {
		jsonError(w, http.StatusNotFound, "file not found")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.WriteHeader(http.StatusOK)
	streamBody(w, rc)
	metrics.ConanDownloadsTotal.WithLabelValues(repo).Inc()

	actor := ""
	if c := claimsFromContext(r.Context()); c != nil {
		actor = c.Username
	}
	go s.store.AppendAudit(storage.AuditEntry{
		Action:  "download",
		Repo:    repo,
		Package: fmt.Sprintf("%s/%s@%s/%s", name, version, namespace, channel),
		Username: actor,
		IP:      r.RemoteAddr,
	})
}

// PUT /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/files/{filename...}
func (s *Server) handleUploadRecipeFile(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel := recipeParams(r)
	rrev := r.PathValue("rrev")
	filename := r.PathValue("filename")

	// Enforce repository-configured namespace / channel constraints
	if err := validateConanRef(r, namespace, channel); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Buffer the body so we can both store it and pass it to git sync.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	if err := s.store.PutRecipeFile(repo, name, version, namespace, channel, rrev, filename, bytes.NewReader(body)); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.AddRecipeRevision(repo, name, version, namespace, channel, rrev); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	metrics.ConanUploadsTotal.WithLabelValues(repo).Inc()

	actor := ""
	if c := claimsFromContext(r.Context()); c != nil {
		actor = c.Username
	}
	pkg := fmt.Sprintf("%s/%s@%s/%s", name, version, namespace, channel)
	go s.store.AppendAudit(storage.AuditEntry{
		Action:  "upload",
		Repo:    repo,
		Package: pkg,
		Username: actor,
		IP:      r.RemoteAddr,
	})
	go s.DispatchWebhook(repo, "package.upload", pkg, actor)

	// Async git sync — fire and forget; errors are logged but don't affect the response.
	go s.syncRecipeFileToGit(repo, name, version, namespace, channel, rrev, filename, body)
}

// DELETE /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}
func (s *Server) handleDeleteRecipeRevision(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel := recipeParams(r)
	rrev := r.PathValue("rrev")

	if !s.store.RecipeRevisionExists(repo, name, version, namespace, channel, rrev) {
		jsonError(w, http.StatusNotFound, "revision not found")
		return
	}
	if err := s.store.DeleteRecipeRevision(repo, name, version, namespace, channel, rrev); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func recipeParams(r *http.Request) (repo, name, version, namespace, channel string) {
	return r.PathValue("repository"), r.PathValue("name"), r.PathValue("version"),
		r.PathValue("namespace"), r.PathValue("channel")
}

// validateConanRef checks whether the @namespace and channel in the request URL
// are within the repository's allowed lists (if configured).
func validateConanRef(r *http.Request, namespace, channel string) error {
	repo := repoFromContext(r.Context())
	if repo == nil {
		return nil
	}
	if len(repo.AllowedNamespaces) > 0 && !contains(repo.AllowedNamespaces, namespace) {
		return fmt.Errorf("namespace %q is not allowed in this repository (allowed: %v)", namespace, repo.AllowedNamespaces)
	}
	if len(repo.AllowedChannels) > 0 && !contains(repo.AllowedChannels, channel) {
		return fmt.Errorf("channel %q is not allowed in this repository (allowed: %v)", channel, repo.AllowedChannels)
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// syncRecipeFileToGit pushes a single uploaded recipe file to the repository's
// configured git remote (if any). Run as a goroutine; errors are only logged.
func (s *Server) syncRecipeFileToGit(repoName, name, version, ns, ch, rrev, filename string, content []byte) {
	rec, err := s.store.GetRepo(repoName)
	if err != nil || rec == nil || rec.Git == nil || rec.Git.URL == "" {
		return // git sync not configured for this repo
	}

	cloneDir := filepath.Join(s.gitWorkspace, repoName)
	syncer := gitops.New(cloneDir, gitops.Config{
		URL:    rec.Git.URL,
		Branch: rec.Git.Branch,
		Token:  rec.Git.Token,
	})

	sha, err := syncer.SyncFile(name, version, ns, ch, rrev, filename, content)
	if err != nil {
		log.Printf("[gitops] %s/%s@%s/%s rrev=%s file=%s: %v", name, version, ns, ch, rrev, filename, err)
		return
	}
	if sha != "" {
		log.Printf("[gitops] %s/%s@%s/%s rrev=%s file=%s synced → %s", name, version, ns, ch, rrev, filename, sha)
	}
}
