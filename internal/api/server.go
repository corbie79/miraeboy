package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

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
	s.registerRoutes()
	return s
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

	// ── Health ──────────────────────────────────────────────────────────────
	m.HandleFunc("GET /ping", s.handlePing)

	// ── Auth ─────────────────────────────────────────────────────────────────
	m.HandleFunc("GET /v2/users/authenticate", s.handleAuthenticate)
	m.HandleFunc("GET /v2/users/check_credentials", s.auth(s.handleCheckCredentials))

	// ── Recipe search ────────────────────────────────────────────────────────
	m.HandleFunc("GET /v2/conans/search", s.auth(s.handleRecipeSearch))

	// ── Recipe revisions ─────────────────────────────────────────────────────
	ref := "/v2/conans/{name}/{version}/{username}/{channel}"

	m.HandleFunc("GET "+ref+"/revisions", s.auth(s.handleListRecipeRevisions))
	m.HandleFunc("GET "+ref+"/revisions/latest", s.auth(s.handleLatestRecipeRevision))

	// ── Recipe files ─────────────────────────────────────────────────────────
	m.HandleFunc("GET "+ref+"/revisions/{rrev}/files", s.auth(s.handleListRecipeFiles))
	m.HandleFunc("GET "+ref+"/revisions/{rrev}/files/{filename...}", s.auth(s.handleDownloadRecipeFile))
	m.HandleFunc("PUT "+ref+"/revisions/{rrev}/files/{filename...}", s.auth(s.handleUploadRecipeFile))
	m.HandleFunc("DELETE "+ref+"/revisions/{rrev}", s.auth(s.handleDeleteRecipeRevision))

	// ── Package revisions ─────────────────────────────────────────────────────
	pkg := ref + "/revisions/{rrev}/packages/{pkgid}"

	m.HandleFunc("GET "+pkg+"/revisions", s.auth(s.handleListPackageRevisions))
	m.HandleFunc("GET "+pkg+"/revisions/latest", s.auth(s.handleLatestPackageRevision))

	// ── Package files ─────────────────────────────────────────────────────────
	m.HandleFunc("GET "+pkg+"/revisions/{prev}/files", s.auth(s.handleListPackageFiles))
	m.HandleFunc("GET "+pkg+"/revisions/{prev}/files/{filename...}", s.auth(s.handleDownloadPackageFile))
	m.HandleFunc("PUT "+pkg+"/revisions/{prev}/files/{filename...}", s.auth(s.handleUploadPackageFile))
	m.HandleFunc("DELETE "+pkg+"/revisions/{prev}", s.auth(s.handleDeletePackageRevision))
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

// loggingMiddleware logs each request.
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
