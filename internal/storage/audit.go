package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// AuditEntry records a single auditable action.
type AuditEntry struct {
	Timestamp time.Time `json:"ts"`
	Username  string    `json:"user"`
	Action    string    `json:"action"` // upload, download, delete, login, repo_create, repo_delete, user_create, user_delete, yank, unyank
	Repo      string    `json:"repo,omitempty"`
	Package   string    `json:"pkg,omitempty"` // "name/version@ns/ch" for Conan, "name/version" for Cargo
	IP        string    `json:"ip,omitempty"`
	Detail    string    `json:"detail,omitempty"` // extra context
}

func auditKeyForMonth(t time.Time) string {
	return fmt.Sprintf("_audit/%s.jsonl", t.UTC().Format("2006-01"))
}

// AppendAudit writes a single audit entry (appends JSONL line).
func (s *Storage) AppendAudit(e AuditEntry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()

	key := auditKeyForMonth(e.Timestamp)
	existing, _ := s.b.Get(key)
	return s.b.Put(key, append(existing, line...))
}

// AuditFilter controls which audit entries are returned.
type AuditFilter struct {
	Repo     string    // filter by repo (empty = all)
	Username string    // filter by user (empty = all)
	Action   string    // filter by action (empty = all)
	Since    time.Time // only entries at or after this time
	Until    time.Time // only entries before this time
	Limit    int       // max entries to return (0 = 200)
	Page     int       // 1-based page number (0 = 1)
}

// QueryAudit returns audit entries matching the filter, newest first.
func (s *Storage) QueryAudit(f AuditFilter) ([]AuditEntry, int, error) {
	if f.Limit <= 0 {
		f.Limit = 200
	}
	if f.Page <= 0 {
		f.Page = 1
	}

	s.mu.RLock()
	keys, err := s.b.List("_audit/")
	s.mu.RUnlock()
	if err != nil {
		keys = []string{}
	}

	// Sort keys newest month first
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	var all []AuditEntry
	for _, key := range keys {
		if !strings.HasSuffix(key, ".jsonl") {
			continue
		}
		s.mu.RLock()
		data, err := s.b.Get(key)
		s.mu.RUnlock()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var e AuditEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				continue
			}
			// Apply filters
			if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
				continue
			}
			if !f.Until.IsZero() && !e.Timestamp.Before(f.Until) {
				continue
			}
			if f.Repo != "" && e.Repo != f.Repo {
				continue
			}
			if f.Username != "" && e.Username != f.Username {
				continue
			}
			if f.Action != "" && e.Action != f.Action {
				continue
			}
			all = append(all, e)
		}
	}

	// Sort newest first
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})

	total := len(all)
	offset := (f.Page - 1) * f.Limit
	if offset >= total {
		return []AuditEntry{}, total, nil
	}
	end := offset + f.Limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

// ensure io is used
var _ = io.Discard
