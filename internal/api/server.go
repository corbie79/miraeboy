package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/config"
	"github.com/corbie79/miraeboy/internal/storage"
)

type Server struct {
	cfg      *config.Config
	store    *storage.Storage
	mux      *http.ServeMux
	webFS    fs.FS        // compiled web UI assets (may be nil)
	nodeRole string       // "primary" or "replica"
	oidc     *oidcProvider
	builds   *BuildStore  // nil when build system is disabled
}

func NewServer(cfg *config.Config, store *storage.Storage, webFS fs.FS) *Server {
	role := cfg.Server.NodeRole
	if role == "" {
		role = "primary"
	}
	s := &Server{
		cfg:      cfg,
		store:    store,
		mux:      http.NewServeMux(),
		webFS:    webFS,
		nodeRole: role,
		oidc:     newOIDCProvider(),
	}
	if cfg.Build.AgentKey != "" {
		s.builds = newBuildStore(cfg.Build.ArtifactsDir)
		log.Printf("Build system enabled (artifacts: %s)", s.builds.artifactsDir)
	}
	s.seedRepos()
	s.registerRoutes()
	return s
}

// seedRepos creates repositories from config.yaml if they don't already exist on disk.
func (s *Server) seedRepos() {
	for _, rdef := range s.cfg.Repositories {
		members := make([]storage.RepoMember, len(rdef.Members))
		for i, m := range rdef.Members {
			members[i] = storage.RepoMember{
				Username:   m.Username,
				Permission: m.Permission,
			}
		}
		if err := s.store.SeedRepo(storage.RepoRecord{
			Name:              rdef.Name,
			Description:       rdef.Description,
			Owner:             rdef.Owner,
			AllowedNamespaces: rdef.AllowedNamespaces,
			AllowedChannels:   rdef.AllowedChannels,
			AnonymousAccess:   rdef.AnonymousAccess,
			Source:            "config",
			CreatedAt:         time.Now().UTC(),
			Members:           members,
		}); err != nil {
			log.Printf("warn: failed to seed repository %q: %v", rdef.Name, err)
		}
	}
}

func (s *Server) Run(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(s.mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) registerRoutes() {
	m := s.mux

	// ── Web UI static files ───────────────────────────────────────────────────
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		m.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: serve index.html for non-asset paths
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" || !strings.Contains(path, ".") {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	// ── Web UI auth ───────────────────────────────────────────────────────────
	m.HandleFunc("POST /api/auth/login", s.handleWebLogin)

	// ── OIDC SSO ──────────────────────────────────────────────────────────────
	m.HandleFunc("GET /api/auth/oidc/status", s.handleOIDCStatus)
	m.HandleFunc("GET /api/auth/oidc/login", s.handleOIDCLogin)
	m.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)

	// ── Global health check ───────────────────────────────────────────────────
	m.HandleFunc("GET /ping", s.handlePing)

	// ── Build system (disabled when build.agent_key is empty) ────────────────
	if s.builds != nil {
		m.HandleFunc("POST /api/builds",                        s.adminOnly(s.handleTriggerBuild))
		m.HandleFunc("GET /api/builds",                         s.adminOnly(s.handleListBuilds))
		m.HandleFunc("GET /api/builds/{id}",                    s.adminOnly(s.handleGetBuild))
		m.HandleFunc("GET /api/builds/{id}/artifacts/{file}",   s.adminOnly(s.handleDownloadArtifact))
		m.HandleFunc("POST /api/agent/poll",                    s.handleAgentPoll)
		m.HandleFunc("POST /api/agent/jobs/{id}/done",          s.handleAgentDone)
	}

	// ── Repository management API ─────────────────────────────────────────────
	m.HandleFunc("POST /api/repos", s.replicaReadOnly(s.adminOnly(s.handleCreateRepo)))
	m.HandleFunc("GET /api/repos", s.adminOnly(s.handleListRepos))
	m.HandleFunc("GET /api/repos/{repository}", s.adminOnly(s.handleGetRepo))
	m.HandleFunc("PATCH /api/repos/{repository}", s.replicaReadOnly(s.requireRepoOwnerOrAdmin(s.handleUpdateRepo)))
	m.HandleFunc("DELETE /api/repos/{repository}", s.replicaReadOnly(s.adminOnly(s.handleDeleteRepo)))

	// ── Repository member management ──────────────────────────────────────────
	m.HandleFunc("POST /api/repos/{repository}/members", s.replicaReadOnly(s.requireRepoOwnerOrAdmin(s.handleInviteMember)))
	m.HandleFunc("GET /api/repos/{repository}/members", s.requireRepoOwnerOrAdmin(s.handleListMembers))
	m.HandleFunc("PUT /api/repos/{repository}/members/{username}", s.replicaReadOnly(s.requireRepoOwnerOrAdmin(s.handleUpdateMember)))
	m.HandleFunc("DELETE /api/repos/{repository}/members/{username}", s.replicaReadOnly(s.requireRepoOwnerOrAdmin(s.handleRemoveMember)))

	// ── Conan v2 endpoints ────────────────────────────────────────────────────
	// Conan client remote URL: http://server:9300/api/conan/{repository}
	s.registerConanRoutes(m, "/api/conan")

	// ── JFrog Artifactory 호환 URL (선택) ────────────────────────────────────
	// artifactory_compat: true 시 /artifactory/api/conan/{repository}/... 도 동작
	// Conan 클라이언트에서 http://server:9300/artifactory/api/conan/extralib 으로 등록 가능
	if s.cfg.Server.ArtifactoryCompat {
		s.registerConanRoutes(m, "/artifactory/api/conan")
		log.Printf("Artifactory-compat URLs enabled: /artifactory/api/conan/...")
	}
}

// registerConanRoutes registers all Conan v2 endpoints under the given base prefix.
// Called twice when artifactory_compat is enabled: once for /api/conan, once for /artifactory/api/conan.
func (s *Server) registerConanRoutes(m *http.ServeMux, base string) {
	m.HandleFunc("GET "+base+"/{repository}/v2/ping", s.handlePing)
	m.HandleFunc("GET "+base+"/{repository}/v2/users/authenticate", s.handleAuthenticate)
	m.HandleFunc("GET "+base+"/{repository}/v2/users/check_credentials",
		s.requirePermission(auth.PermRead, s.handleCheckCredentials))

	m.HandleFunc("GET "+base+"/{repository}/v2/conans/search",
		s.requirePermission(auth.PermRead, s.handleRecipeSearch))

	ref := base + "/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}"

	m.HandleFunc("GET "+ref+"/revisions",
		s.requirePermission(auth.PermRead, s.handleListRecipeRevisions))
	m.HandleFunc("GET "+ref+"/revisions/latest",
		s.requirePermission(auth.PermRead, s.handleLatestRecipeRevision))
	m.HandleFunc("GET "+ref+"/revisions/{rrev}/files",
		s.requirePermission(auth.PermRead, s.handleListRecipeFiles))
	m.HandleFunc("GET "+ref+"/revisions/{rrev}/files/{filename...}",
		s.requirePermission(auth.PermRead, s.handleDownloadRecipeFile))
	m.HandleFunc("PUT "+ref+"/revisions/{rrev}/files/{filename...}",
		s.replicaReadOnly(s.requirePermission(auth.PermWrite, s.handleUploadRecipeFile)))
	m.HandleFunc("DELETE "+ref+"/revisions/{rrev}",
		s.replicaReadOnly(s.requirePermission(auth.PermDelete, s.handleDeleteRecipeRevision)))

	pkg := ref + "/revisions/{rrev}/packages/{pkgid}"

	m.HandleFunc("GET "+pkg+"/revisions",
		s.requirePermission(auth.PermRead, s.handleListPackageRevisions))
	m.HandleFunc("GET "+pkg+"/revisions/latest",
		s.requirePermission(auth.PermRead, s.handleLatestPackageRevision))
	m.HandleFunc("GET "+pkg+"/revisions/{prev}/files",
		s.requirePermission(auth.PermRead, s.handleListPackageFiles))
	m.HandleFunc("GET "+pkg+"/revisions/{prev}/files/{filename...}",
		s.requirePermission(auth.PermRead, s.handleDownloadPackageFile))
	m.HandleFunc("PUT "+pkg+"/revisions/{prev}/files/{filename...}",
		s.replicaReadOnly(s.requirePermission(auth.PermWrite, s.handleUploadPackageFile)))
	m.HandleFunc("DELETE "+pkg+"/revisions/{prev}",
		s.replicaReadOnly(s.requirePermission(auth.PermDelete, s.handleDeletePackageRevision)))
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
