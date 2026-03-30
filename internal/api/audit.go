package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/corbie79/miraeboy/internal/storage"
)

// GET /api/audit  (admin only)
// Query params: repo=, user=, action=, since=RFC3339, until=RFC3339, page=, limit=
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := storage.AuditFilter{
		Repo:     q.Get("repo"),
		Username: q.Get("user"),
		Action:   q.Get("action"),
	}

	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Until = t
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 1000 {
				n = 1000
			}
			f.Limit = n
		}
	}
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			f.Page = n
		}
	}

	entries, total, err := s.store.QueryAudit(f)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []storage.AuditEntry{}
	}

	pages := total / f.Limit
	if total%f.Limit != 0 {
		pages++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
		"page":    f.Page,
		"pages":   pages,
	})
}
