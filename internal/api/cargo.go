package api

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/corbie79/miraeboy/internal/auth"
	"github.com/corbie79/miraeboy/internal/storage"
)

// registerCargoRoutes mounts the Cargo sparse registry under /cargo/{repository}/.
//
// Cargo client configuration (~/.cargo/config.toml):
//
//	[registries]
//	my-company = { index = "sparse+http://miraeboy.example.com:9300/cargo/myrepo/" }
//
//	[registry]
//	token = "Bearer <JWT from mboy login>"   # or just the JWT without "Bearer "
func (s *Server) registerCargoRoutes(m *http.ServeMux) {
	base := "/cargo/{repository}"

	// ── index (sparse registry protocol) ──────────────────────────────────────
	m.HandleFunc("GET "+base+"/index/config.json",
		s.cargoAuth(auth.PermRead, s.handleCargoIndexConfig))
	m.HandleFunc("GET "+base+"/index/{prefix}/{crate}",
		s.cargoAuth(auth.PermRead, s.handleCargoIndexEntry))

	// ── crate API ─────────────────────────────────────────────────────────────
	m.HandleFunc("GET "+base+"/api/v1/crates",
		s.cargoAuth(auth.PermRead, s.handleCargoSearch))
	m.HandleFunc("PUT "+base+"/api/v1/crates/new",
		s.replicaReadOnly(s.cargoAuth(auth.PermWrite, s.handleCargoPublish)))
	m.HandleFunc("GET "+base+"/api/v1/crates/{name}/{version}/download",
		s.cargoAuth(auth.PermRead, s.handleCargoDownload))
	m.HandleFunc("DELETE "+base+"/api/v1/crates/{name}/{version}/yank",
		s.replicaReadOnly(s.cargoAuth(auth.PermDelete, s.handleCargoYank)))
	m.HandleFunc("PUT "+base+"/api/v1/crates/{name}/{version}/unyank",
		s.replicaReadOnly(s.cargoAuth(auth.PermWrite, s.handleCargoUnyank)))
}

// ─── auth middleware ──────────────────────────────────────────────────────────

// cargoAuth validates Cargo's "Authorization: <token>" header.
// Cargo sends the token without a "Bearer" prefix by default; we support both.
func (s *Server) cargoAuth(perm auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo := r.PathValue("repository")

		raw := r.Header.Get("Authorization")
		token := strings.TrimPrefix(raw, "Bearer ")
		token = strings.TrimSpace(token)

		if token != "" {
			claims, err := auth.ValidateToken(s.cfg.Auth.JWTSecret, token)
			if err == nil {
				if !claims.GroupPermission(repo).Satisfies(perm) {
					cargoError(w, http.StatusForbidden, "insufficient permissions")
					return
				}
				r = r.WithContext(contextWithClaims(r.Context(), claims))
				// also store repo record in context
				rec, _ := s.store.GetRepo(repo)
				if rec != nil {
					r = r.WithContext(contextWithRepo(r.Context(), rec))
				}
				next(w, r)
				return
			}
		}

		// Check anonymous access.
		rec, _ := s.store.GetRepo(repo)
		if rec != nil && perm == auth.PermRead &&
			(rec.AnonymousAccess == "read" || rec.AnonymousAccess == "write") {
			r = r.WithContext(contextWithRepo(r.Context(), rec))
			next(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `cargo-token`)
		cargoError(w, http.StatusUnauthorized, "authentication required")
	}
}

// ─── index endpoints ──────────────────────────────────────────────────────────

// GET /cargo/{repo}/index/config.json
func (s *Server) handleCargoIndexConfig(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	base := fmt.Sprintf("%s://%s/cargo/%s", scheme, r.Host, repo)

	writeJSON(w, http.StatusOK, map[string]any{
		"dl":  base + "/api/v1/crates/{crate}/{version}/download",
		"api": base,
	})
}

// GET /cargo/{repo}/index/{prefix}/{crate}
func (s *Server) handleCargoIndexEntry(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	crate := r.PathValue("crate")

	entries, err := s.store.GetCargoIndex(repo, crate)
	if err != nil {
		cargoError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(entries) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	for _, e := range entries {
		line, _ := json.Marshal(e)
		fmt.Fprintf(w, "%s\n", line)
	}
}

// ─── crate API endpoints ──────────────────────────────────────────────────────

// GET /cargo/{repo}/api/v1/crates?q=&per_page=
func (s *Server) handleCargoSearch(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	query := r.URL.Query().Get("q")
	perPage := 10

	results, err := s.store.SearchCargo(repo, query)
	if err != nil {
		cargoError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(results) > perPage {
		results = results[:perPage]
	}

	type crateResult struct {
		Name        string `json:"name"`
		MaxVersion  string `json:"max_version"`
		Description string `json:"description"`
	}
	crates := make([]crateResult, len(results))
	for i, e := range results {
		crates[i] = crateResult{Name: e.Name, MaxVersion: e.Vers}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"crates": crates,
		"meta":   map[string]int{"total": len(crates)},
	})
}

// PUT /cargo/{repo}/api/v1/crates/new
// Body: 4-byte LE length of JSON metadata + JSON + 4-byte LE length of .crate + .crate bytes
func (s *Server) handleCargoPublish(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		cargoError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if len(body) < 8 {
		cargoError(w, http.StatusBadRequest, "malformed publish request")
		return
	}

	// Parse the Cargo publish wire format.
	metaLen := int(body[0]) | int(body[1])<<8 | int(body[2])<<16 | int(body[3])<<24
	if len(body) < 4+metaLen+4 {
		cargoError(w, http.StatusBadRequest, "malformed publish request: truncated metadata")
		return
	}
	metaJSON := body[4 : 4+metaLen]

	crateOffset := 4 + metaLen
	crateLen := int(body[crateOffset]) | int(body[crateOffset+1])<<8 |
		int(body[crateOffset+2])<<16 | int(body[crateOffset+3])<<24
	if len(body) < crateOffset+4+crateLen {
		cargoError(w, http.StatusBadRequest, "malformed publish request: truncated crate data")
		return
	}
	crateData := body[crateOffset+4 : crateOffset+4+crateLen]

	// Decode metadata.
	var meta struct {
		Name     string      `json:"name"`
		Vers     string      `json:"vers"`
		Deps     []struct {
			Name           string   `json:"name"`
			VersionReq     string   `json:"version_req"`
			Features       []string `json:"features"`
			Optional       bool     `json:"optional"`
			DefaultFeatures bool    `json:"default_features"`
			Target         *string  `json:"target"`
			Kind           string   `json:"kind"`
			Registry       *string  `json:"registry"`
			ExplicitNameInToml *string `json:"explicit_name_in_toml"`
		} `json:"deps"`
		Features map[string][]string `json:"features"`
		Authors  []string            `json:"authors"`
		Descr    string              `json:"description"`
		Links    *string             `json:"links"`
	}
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		cargoError(w, http.StatusBadRequest, "invalid metadata JSON: "+err.Error())
		return
	}
	if meta.Name == "" || meta.Vers == "" {
		cargoError(w, http.StatusBadRequest, "name and vers are required")
		return
	}

	// Validate it's a real .crate (gzipped tar).
	if err := validateCrate(crateData); err != nil {
		cargoError(w, http.StatusBadRequest, "invalid .crate file: "+err.Error())
		return
	}

	// Store .crate file.
	cksum, err := s.store.PutCrateFile(repo, meta.Name, meta.Vers,
		strings.NewReader(string(crateData)))
	if err != nil {
		cargoError(w, http.StatusInternalServerError, "failed to store crate: "+err.Error())
		return
	}

	// Build index entry.
	deps := make([]storage.CargoDep, len(meta.Deps))
	for i, d := range meta.Deps {
		name := d.Name
		if d.ExplicitNameInToml != nil {
			name = *d.ExplicitNameInToml
		}
		deps[i] = storage.CargoDep{
			Name:            name,
			Req:             d.VersionReq,
			Features:        d.Features,
			Optional:        d.Optional,
			DefaultFeatures: d.DefaultFeatures,
			Target:          d.Target,
			Kind:            d.Kind,
			Registry:        d.Registry,
		}
		if d.ExplicitNameInToml != nil {
			deps[i].Package = &d.Name
		}
	}

	feats := meta.Features
	if feats == nil {
		feats = map[string][]string{}
	}

	entry := storage.CargoIndexEntry{
		Name:   meta.Name,
		Vers:   meta.Vers,
		Deps:   deps,
		Cksum:  cksum,
		Feats:  feats,
		Yanked: false,
		Links:  meta.Links,
		V:      1,
	}
	if err := s.store.AppendCargoIndex(repo, entry); err != nil {
		cargoError(w, http.StatusInternalServerError, "failed to update index: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"warnings": map[string]any{
			"invalid_categories": []string{},
			"invalid_badges":     []string{},
			"other":              []string{},
		},
	})
}

// GET /cargo/{repo}/api/v1/crates/{name}/{version}/download
func (s *Server) handleCargoDownload(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	name := r.PathValue("name")
	version := r.PathValue("version")

	rc, size, err := s.store.GetCrateFile(repo, name, version)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s-%s.crate"`, name, version))
	if size > 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	}
	w.WriteHeader(http.StatusOK)
	streamBody(w, rc)
}

// DELETE /cargo/{repo}/api/v1/crates/{name}/{version}/yank
func (s *Server) handleCargoYank(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	name := r.PathValue("name")
	version := r.PathValue("version")

	if err := s.store.SetCargoYanked(repo, name, version, true); err != nil {
		cargoError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// PUT /cargo/{repo}/api/v1/crates/{name}/{version}/unyank
func (s *Server) handleCargoUnyank(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repository")
	name := r.PathValue("name")
	version := r.PathValue("version")

	if err := s.store.SetCargoYanked(repo, name, version, false); err != nil {
		cargoError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func cargoError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"errors": []map[string]string{{"detail": msg}},
	})
}

// validateCrate checks that the data is a valid gzipped tar archive.
func validateCrate(data []byte) error {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("not a gzip file: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	if _, err := tr.Next(); err != nil {
		return fmt.Errorf("empty or invalid tar: %w", err)
	}
	return nil
}
