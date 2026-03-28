package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/config"
	"github.com/corbie79/miraeboy/internal/storage"
)

type Server struct {
	cfg   *config.Config
	store *storage.Storage
	mux   *http.ServeMux
}

func NewServer(cfg *config.Config, store *storage.Storage) *Server {
	s := &Server{
		cfg:   cfg,
		store: store,
		mux:   http.NewServeMux(),
	}
	s.seedGroups()
	s.registerRoutes()
	return s
}

// seedGroups creates groups from config.yaml if they don't already exist on disk.
func (s *Server) seedGroups() {
	for _, gdef := range s.cfg.Groups {
		members := make([]storage.GroupMember, len(gdef.Members))
		for i, m := range gdef.Members {
			members[i] = storage.GroupMember{
				Username:   m.Username,
				Permission: m.Permission,
			}
		}
		if err := s.store.SeedGroup(storage.GroupRecord{
			Name:            gdef.Name,
			Description:     gdef.Description,
			Owner:           gdef.Owner,
			ConanUser:       gdef.ConanUser,
			ConanChannel:    gdef.ConanChannel,
			AnonymousAccess: gdef.AnonymousAccess,
			Source:          "config",
			CreatedAt:       time.Now().UTC(),
			Members:         members,
		}); err != nil {
			log.Printf("warn: failed to seed group %q: %v", gdef.Name, err)
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

	// ── Global health check ───────────────────────────────────────────────────
	m.HandleFunc("GET /ping", s.handlePing)

	// ── Group management API (global, no {group} in path) ────────────────────
	m.HandleFunc("POST /api/groups", s.adminOnly(s.handleCreateGroup))
	m.HandleFunc("GET /api/groups", s.adminOnly(s.handleListGroups))
	m.HandleFunc("GET /api/groups/{group}", s.adminOnly(s.handleGetGroup))
	m.HandleFunc("PATCH /api/groups/{group}", s.requireGroupOwnerOrAdmin(s.handleUpdateGroup))
	m.HandleFunc("DELETE /api/groups/{group}", s.adminOnly(s.handleDeleteGroup))

	// ── Group member management ───────────────────────────────────────────────
	m.HandleFunc("POST /api/groups/{group}/members", s.requireGroupOwnerOrAdmin(s.handleInviteMember))
	m.HandleFunc("GET /api/groups/{group}/members", s.requireGroupOwnerOrAdmin(s.handleListMembers))
	m.HandleFunc("PUT /api/groups/{group}/members/{username}", s.requireGroupOwnerOrAdmin(s.handleUpdateMember))
	m.HandleFunc("DELETE /api/groups/{group}/members/{username}", s.requireGroupOwnerOrAdmin(s.handleRemoveMember))

	// ── Group-scoped Conan v2 endpoints ───────────────────────────────────────
	// Conan client remote URL: http://server:9300/{group}
	m.HandleFunc("GET /{group}/ping", s.handlePing)
	m.HandleFunc("GET /{group}/v2/users/authenticate", s.handleAuthenticate)
	m.HandleFunc("GET /{group}/v2/users/check_credentials",
		s.requirePermission(auth.PermRead, s.handleCheckCredentials))

	m.HandleFunc("GET /{group}/v2/conans/search",
		s.requirePermission(auth.PermRead, s.handleRecipeSearch))

	ref := "/{group}/v2/conans/{name}/{version}/{username}/{channel}"

	m.HandleFunc("GET "+ref+"/revisions",
		s.requirePermission(auth.PermRead, s.handleListRecipeRevisions))
	m.HandleFunc("GET "+ref+"/revisions/latest",
		s.requirePermission(auth.PermRead, s.handleLatestRecipeRevision))
	m.HandleFunc("GET "+ref+"/revisions/{rrev}/files",
		s.requirePermission(auth.PermRead, s.handleListRecipeFiles))
	m.HandleFunc("GET "+ref+"/revisions/{rrev}/files/{filename...}",
		s.requirePermission(auth.PermRead, s.handleDownloadRecipeFile))
	m.HandleFunc("PUT "+ref+"/revisions/{rrev}/files/{filename...}",
		s.requirePermission(auth.PermWrite, s.handleUploadRecipeFile))
	m.HandleFunc("DELETE "+ref+"/revisions/{rrev}",
		s.requirePermission(auth.PermDelete, s.handleDeleteRecipeRevision))

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
		s.requirePermission(auth.PermWrite, s.handleUploadPackageFile))
	m.HandleFunc("DELETE "+pkg+"/revisions/{prev}",
		s.requirePermission(auth.PermDelete, s.handleDeletePackageRevision))
}

// ─── helpers ─────────────────────────────────────────────────────────────────

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
