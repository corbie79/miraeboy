package api

import (
	"fmt"
	"io"
	"net/http"
)

// GET /api/conan/{repository}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/packages/{pkgid}/revisions
func (s *Server) handleListPackageRevisions(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel, rrev, pkgid := pkgParams(r)
	revs, err := s.store.GetPackageRevisions(repo, name, version, namespace, channel, pkgid, rrev)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": revs})
}

// GET .../packages/{pkgid}/revisions/latest
func (s *Server) handleLatestPackageRevision(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel, rrev, pkgid := pkgParams(r)
	revs, err := s.store.GetPackageRevisions(repo, name, version, namespace, channel, pkgid, rrev)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(revs) == 0 {
		jsonError(w, http.StatusNotFound, "no package revisions found")
		return
	}
	writeJSON(w, http.StatusOK, revs[0])
}

// GET .../packages/{pkgid}/revisions/{prev}/files
func (s *Server) handleListPackageFiles(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel, rrev, pkgid := pkgParams(r)
	prev := r.PathValue("prev")

	if !s.store.PackageRevisionExists(repo, name, version, namespace, channel, pkgid, rrev, prev) {
		jsonError(w, http.StatusNotFound, "package revision not found")
		return
	}
	files, err := s.store.ListPackageFiles(repo, name, version, namespace, channel, pkgid, rrev, prev)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

// GET .../packages/{pkgid}/revisions/{prev}/files/{filename...}
func (s *Server) handleDownloadPackageFile(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel, rrev, pkgid := pkgParams(r)
	prev := r.PathValue("prev")
	filename := r.PathValue("filename")

	rc, size, err := s.store.GetPackageFile(repo, name, version, namespace, channel, pkgid, rrev, prev, filename)
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

// PUT .../packages/{pkgid}/revisions/{prev}/files/{filename...}
func (s *Server) handleUploadPackageFile(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel, rrev, pkgid := pkgParams(r)
	prev := r.PathValue("prev")
	filename := r.PathValue("filename")

	// Enforce repository-configured namespace / channel constraints
	if err := validateConanRef(r, namespace, channel); err != nil {
		jsonError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if err := s.store.PutPackageFile(repo, name, version, namespace, channel, pkgid, rrev, prev, filename, r.Body); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.store.AddPackageRevision(repo, name, version, namespace, channel, pkgid, rrev, prev); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// DELETE .../packages/{pkgid}/revisions/{prev}
func (s *Server) handleDeletePackageRevision(w http.ResponseWriter, r *http.Request) {
	repo, name, version, namespace, channel, rrev, pkgid := pkgParams(r)
	prev := r.PathValue("prev")

	if !s.store.PackageRevisionExists(repo, name, version, namespace, channel, pkgid, rrev, prev) {
		jsonError(w, http.StatusNotFound, "package revision not found")
		return
	}
	if err := s.store.DeletePackageRevision(repo, name, version, namespace, channel, pkgid, rrev, prev); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pkgParams(r *http.Request) (repo, name, version, namespace, channel, rrev, pkgid string) {
	return r.PathValue("repository"), r.PathValue("name"), r.PathValue("version"),
		r.PathValue("namespace"), r.PathValue("channel"),
		r.PathValue("rrev"), r.PathValue("pkgid")
}

func streamBody(w http.ResponseWriter, rc io.Reader) {
	buf := make([]byte, 32*1024)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			w.Write(buf[:n]) //nolint:errcheck
		}
		if err != nil {
			break
		}
	}
}
