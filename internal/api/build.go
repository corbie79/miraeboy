package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ─── Model ────────────────────────────────────────────────────────────────────

const defaultBuildPlatforms = "linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64"

type BuildJob struct {
	ID         string     `json:"id"`
	BuildID    string     `json:"build_id"`
	Platform   string     `json:"platform"` // "linux/amd64"
	Status     string     `json:"status"`   // pending | running | success | failed
	AgentID    string     `json:"agent_id,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	Artifact   string     `json:"artifact,omitempty"`
	// Embedded for agent convenience
	RepoURL string `json:"repo_url"`
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

type BuildRecord struct {
	ID        string      `json:"id"`
	RepoURL   string      `json:"repo_url"`
	Ref       string      `json:"ref"`
	Version   string      `json:"version"`
	CreatedAt time.Time   `json:"created_at"`
	Status    string      `json:"status"`
	Jobs      []*BuildJob `json:"jobs"`
}

func buildStatus(jobs []*BuildJob) string {
	counts := map[string]int{}
	for _, j := range jobs {
		counts[j.Status]++
	}
	switch {
	case counts["failed"] > 0:
		return "failed"
	case counts["running"] > 0:
		return "running"
	case counts["pending"] > 0:
		return "pending"
	case len(jobs) > 0:
		return "success"
	default:
		return "pending"
	}
}

// ─── BuildStore ───────────────────────────────────────────────────────────────

type BuildStore struct {
	mu           sync.Mutex
	builds       map[string]*BuildRecord
	jobs         map[string]*BuildJob
	artifactsDir string
}

func newBuildStore(artifactsDir string) *BuildStore {
	if artifactsDir == "" {
		artifactsDir = "./artifacts"
	}
	os.MkdirAll(artifactsDir, 0o755)
	return &BuildStore{
		builds:       make(map[string]*BuildRecord),
		jobs:         make(map[string]*BuildJob),
		artifactsDir: artifactsDir,
	}
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// POST /api/builds — trigger a new build (admin only)
func (s *Server) handleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RepoURL   string   `json:"repo_url"`
		Ref       string   `json:"ref"`
		Version   string   `json:"version"`
		Platforms []string `json:"platforms"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.RepoURL == "" {
		jsonError(w, http.StatusBadRequest, "repo_url is required")
		return
	}
	if req.Ref == "" {
		req.Ref = "main"
	}
	if req.Version == "" {
		req.Version = "dev"
	}
	if len(req.Platforms) == 0 {
		req.Platforms = strings.Split(defaultBuildPlatforms, ",")
	}

	b := &BuildRecord{
		ID:        newID(),
		RepoURL:   req.RepoURL,
		Ref:       req.Ref,
		Version:   req.Version,
		CreatedAt: time.Now(),
	}

	s.builds.mu.Lock()
	for _, p := range req.Platforms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		j := &BuildJob{
			ID:      newID(),
			BuildID: b.ID,
			Platform: p,
			Status:  "pending",
			RepoURL: req.RepoURL,
			Ref:     req.Ref,
			Version: req.Version,
		}
		b.Jobs = append(b.Jobs, j)
		s.builds.jobs[j.ID] = j
	}
	b.Status = buildStatus(b.Jobs)
	s.builds.builds[b.ID] = b
	s.builds.mu.Unlock()

	log.Printf("build %s: %s@%s version=%s jobs=%d", b.ID, req.RepoURL, req.Ref, req.Version, len(b.Jobs))
	writeJSON(w, http.StatusCreated, b)
}

// GET /api/builds — list all builds (admin only)
func (s *Server) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	s.builds.mu.Lock()
	list := make([]*BuildRecord, 0, len(s.builds.builds))
	for _, b := range s.builds.builds {
		b.Status = buildStatus(b.Jobs)
		list = append(list, b)
	}
	s.builds.mu.Unlock()
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, list)
}

// GET /api/builds/{id} — get build detail (admin only)
func (s *Server) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	s.builds.mu.Lock()
	b := s.builds.builds[r.PathValue("id")]
	if b != nil {
		b.Status = buildStatus(b.Jobs)
	}
	s.builds.mu.Unlock()
	if b == nil {
		jsonError(w, http.StatusNotFound, "build not found")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// GET /api/builds/{id}/artifacts/{file} — download artifact (admin only)
func (s *Server) handleDownloadArtifact(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if strings.ContainsAny(file, "/\\") || strings.Contains(file, "..") {
		jsonError(w, http.StatusBadRequest, "invalid filename")
		return
	}
	path := filepath.Join(s.builds.artifactsDir, r.PathValue("id"), file)
	f, err := os.Open(path)
	if err != nil {
		jsonError(w, http.StatusNotFound, "artifact not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", "attachment; filename="+file)
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, f)
}

// POST /api/agent/poll — miraeboy-agent polls for next pending job
// Auth: X-Agent-Key header (build.agent_key in config.yaml)
func (s *Server) handleAgentPoll(w http.ResponseWriter, r *http.Request) {
	if !s.checkAgentKey(r) {
		jsonError(w, http.StatusUnauthorized, "invalid agent key")
		return
	}
	var req struct {
		AgentID  string `json:"agent_id"`
		Platform string `json:"platform"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Platform == "" {
		jsonError(w, http.StatusBadRequest, "platform required")
		return
	}

	s.builds.mu.Lock()
	var found *BuildJob
	for _, j := range s.builds.jobs {
		if j.Status == "pending" && j.Platform == req.Platform {
			found = j
			break
		}
	}
	if found != nil {
		now := time.Now()
		found.Status = "running"
		found.AgentID = req.AgentID
		found.StartedAt = &now
	}
	s.builds.mu.Unlock()

	if found == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	log.Printf("build-job %s → agent %s (%s)", found.ID, req.AgentID, req.Platform)
	writeJSON(w, http.StatusOK, found)
}

// POST /api/agent/jobs/{id}/done — miraeboy-agent submits result + artifact
func (s *Server) handleAgentDone(w http.ResponseWriter, r *http.Request) {
	if !s.checkAgentKey(r) {
		jsonError(w, http.StatusUnauthorized, "invalid agent key")
		return
	}

	s.builds.mu.Lock()
	job := s.builds.jobs[r.PathValue("id")]
	s.builds.mu.Unlock()
	if job == nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}

	if err := r.ParseMultipartForm(1 << 30); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	status := r.FormValue("status")
	errMsg := r.FormValue("error")
	now := time.Now()

	s.builds.mu.Lock()
	job.Status = status
	job.Error = errMsg
	job.FinishedAt = &now
	s.builds.mu.Unlock()

	if status == "success" {
		if f, h, err := r.FormFile("artifact"); err == nil {
			defer f.Close()
			dir := filepath.Join(s.builds.artifactsDir, job.BuildID)
			os.MkdirAll(dir, 0o755)
			if dst, err := os.Create(filepath.Join(dir, h.Filename)); err == nil {
				io.Copy(dst, f)
				dst.Close()
				s.builds.mu.Lock()
				job.Artifact = h.Filename
				s.builds.mu.Unlock()
				log.Printf("build-job %s: artifact saved: %s", job.ID, h.Filename)
			}
		}
	}

	log.Printf("build-job %s: %s (agent=%s)", job.ID, status, job.AgentID)
	w.WriteHeader(http.StatusNoContent)
}

// checkAgentKey validates the X-Agent-Key header against config.
func (s *Server) checkAgentKey(r *http.Request) bool {
	key := s.cfg.Build.AgentKey
	return key != "" && r.Header.Get("X-Agent-Key") == key
}
