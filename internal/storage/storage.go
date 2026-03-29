package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// Revision represents a single revision entry stored in revisions.json.
type Revision struct {
	Revision string    `json:"revision"`
	Time     time.Time `json:"time"`
}

// RepoMember holds a user's permission within a repository.
type RepoMember struct {
	Username   string `json:"username"`
	Permission string `json:"permission"` // "read", "write", "delete", "owner"
}

// GitSyncConfig holds the git sync settings for a single repository.
// Leave URL empty to disable git sync for this repository.
type GitSyncConfig struct {
	URL    string `json:"url"`    // HTTPS clone URL of the target git repo
	Branch string `json:"branch"` // target branch (default: "main")
	Token  string `json:"token"`  // HTTPS auth token (PAT, Gitea key, …)
}

// RepoRecord is the full definition of a Conan repository.
// Stored as _repos/{name}.json in the backend.
type RepoRecord struct {
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Owner             string        `json:"owner"`
	AllowedNamespaces []string      `json:"allowed_namespaces"`
	AllowedChannels   []string      `json:"allowed_channels"`
	AnonymousAccess   string        `json:"anonymous_access"`
	Source            string        `json:"source"`
	CreatedAt         time.Time     `json:"created_at"`
	Members           []RepoMember  `json:"members"`
	Git               *GitSyncConfig `json:"git,omitempty"` // optional git sync config
}

// Storage is the primary entry point for all package and repository operations.
// It wraps a Backend (filesystem or S3) and serialises writes with a mutex.
type Storage struct {
	b  Backend
	mu sync.RWMutex
}

// New creates a Storage backed by the local filesystem at base.
func New(base string) (*Storage, error) {
	b, err := NewFSBackend(base)
	if err != nil {
		return nil, err
	}
	return &Storage{b: b}, nil
}

// NewWithBackend creates a Storage using the provided Backend.
func NewWithBackend(b Backend) *Storage {
	return &Storage{b: b}
}

// ─── key helpers ──────────────────────────────────────────────────────────────

func repoMetaKey(name string) string { return "_repos/" + name + ".json" }
func reposPrefix() string            { return "_repos/" }

func recipeRevKey(repo, name, version, ns, ch string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/recipe_revisions.json", repo, name, version, ns, ch)
}
func recipeRevDirPrefix(repo, name, version, ns, ch, rrev string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s/", repo, name, version, ns, ch, rrev)
}
func recipeFileKey(repo, name, version, ns, ch, rrev, filename string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s", repo, name, version, ns, ch, rrev, filename)
}

func pkgRevKey(repo, name, version, ns, ch, pkgid, rrev string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/packages/%s/%s/pkg_revisions.json",
		repo, name, version, ns, ch, pkgid, rrev)
}
func pkgRevDirPrefix(repo, name, version, ns, ch, pkgid, rrev, prev string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/packages/%s/%s/%s/",
		repo, name, version, ns, ch, pkgid, rrev, prev)
}
func pkgFileKey(repo, name, version, ns, ch, pkgid, rrev, prev, filename string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/packages/%s/%s/%s/%s",
		repo, name, version, ns, ch, pkgid, rrev, prev, filename)
}

// ─── repository registry ──────────────────────────────────────────────────────

func (s *Storage) RepoExists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.b.Exists(repoMetaKey(name))
}

func (s *Storage) GetRepo(name string) (*RepoRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.b.Get(repoMetaKey(name))
	if err != nil {
		return nil, nil
	}
	var r RepoRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Storage) ListRepos() ([]RepoRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.b.List(reposPrefix())
	if err != nil {
		return []RepoRecord{}, nil
	}

	var repos []RepoRecord
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}
		data, err := s.b.Get(key)
		if err != nil {
			continue
		}
		var r RepoRecord
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		repos = append(repos, r)
	}
	return repos, nil
}

func (s *Storage) SaveRepo(r RepoRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return putJSON(s.b, repoMetaKey(r.Name), r)
}

func (s *Storage) SeedRepo(r RepoRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.b.Exists(repoMetaKey(r.Name)) {
		return nil
	}
	return putJSON(s.b, repoMetaKey(r.Name), r)
}

func (s *Storage) DeleteRepo(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.b.Delete(repoMetaKey(name)); err != nil {
		return err
	}
	return s.b.DeletePrefix(name + "/")
}

func (s *Storage) GetUserRepoPermissions(username string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.b.List(reposPrefix())
	if err != nil {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}
		data, err := s.b.Get(key)
		if err != nil {
			continue
		}
		var r RepoRecord
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		if r.Owner == username {
			result[r.Name] = "owner"
			continue
		}
		for _, m := range r.Members {
			if m.Username == username {
				result[r.Name] = m.Permission
				break
			}
		}
	}
	return result, nil
}

// ─── user registry ────────────────────────────────────────────────────────────

// UserRecord is a server account stored in _users/{username}.json.
type UserRecord struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"` // sha256 hex of password
	Admin        bool      `json:"admin"`
	CreatedAt    time.Time `json:"created_at"`
	Source       string    `json:"source"` // "config" or "api"
}

func userMetaKey(username string) string { return "_users/" + username + ".json" }
func usersPrefix() string                { return "_users/" }

func (s *Storage) GetUser(username string) (*UserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := s.b.Get(userMetaKey(username))
	if err != nil {
		return nil, nil
	}
	var u UserRecord
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Storage) ListUsers() ([]UserRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys, err := s.b.List(usersPrefix())
	if err != nil {
		return []UserRecord{}, nil
	}
	var users []UserRecord
	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}
		data, err := s.b.Get(key)
		if err != nil {
			continue
		}
		var u UserRecord
		if err := json.Unmarshal(data, &u); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *Storage) SaveUser(u UserRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return putJSON(s.b, userMetaKey(u.Username), u)
}

func (s *Storage) SeedUser(u UserRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.b.Exists(userMetaKey(u.Username)) {
		return nil
	}
	return putJSON(s.b, userMetaKey(u.Username), u)
}

func (s *Storage) DeleteUser(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Delete(userMetaKey(username))
}

func (s *Storage) UserExists(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.b.Exists(userMetaKey(username))
}

// FindUser returns the user record if username and passwordHash match.
func (s *Storage) FindUser(username, passwordHash string) (*UserRecord, error) {
	u, err := s.GetUser(username)
	if err != nil || u == nil {
		return nil, err
	}
	if u.PasswordHash != passwordHash {
		return nil, nil
	}
	return u, nil
}

func (s *Storage) GetRecipeRevisions(repo, name, version, ns, ch string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.b, recipeRevKey(repo, name, version, ns, ch))
}

func (s *Storage) AddRecipeRevision(repo, name, version, ns, ch, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return appendRevision(s.b, recipeRevKey(repo, name, version, ns, ch), rrev)
}

func (s *Storage) DeleteRecipeRevision(repo, name, version, ns, ch, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.b.DeletePrefix(recipeRevDirPrefix(repo, name, version, ns, ch, rrev)); err != nil {
		return err
	}
	return removeRevision(s.b, recipeRevKey(repo, name, version, ns, ch), rrev)
}

func (s *Storage) RecipeRevisionExists(repo, name, version, ns, ch, rrev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.b, recipeRevKey(repo, name, version, ns, ch))
	if err != nil {
		return false
	}
	for _, r := range revs {
		if r.Revision == rrev {
			return true
		}
	}
	return false
}

// ─── recipe files ─────────────────────────────────────────────────────────────

func (s *Storage) ListRecipeFiles(repo, name, version, ns, ch, rrev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listFilesUnderPrefix(recipeRevDirPrefix(repo, name, version, ns, ch, rrev))
}

func (s *Storage) GetRecipeFile(repo, name, version, ns, ch, rrev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, err := safeKey(recipeRevDirPrefix(repo, name, version, ns, ch, rrev) + filename)
	if err != nil {
		return nil, 0, err
	}
	return s.b.GetStream(key)
}

func (s *Storage) PutRecipeFile(repo, name, version, ns, ch, rrev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, err := safeKey(recipeRevDirPrefix(repo, name, version, ns, ch, rrev) + filename)
	if err != nil {
		return err
	}
	return s.b.PutStream(key, r, -1)
}

// ─── package revisions ────────────────────────────────────────────────────────

func (s *Storage) GetPackageRevisions(repo, name, version, ns, ch, pkgid, rrev string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.b, pkgRevKey(repo, name, version, ns, ch, pkgid, rrev))
}

func (s *Storage) AddPackageRevision(repo, name, version, ns, ch, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return appendRevision(s.b, pkgRevKey(repo, name, version, ns, ch, pkgid, rrev), prev)
}

func (s *Storage) DeletePackageRevision(repo, name, version, ns, ch, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.b.DeletePrefix(pkgRevDirPrefix(repo, name, version, ns, ch, pkgid, rrev, prev)); err != nil {
		return err
	}
	return removeRevision(s.b, pkgRevKey(repo, name, version, ns, ch, pkgid, rrev), prev)
}

func (s *Storage) PackageRevisionExists(repo, name, version, ns, ch, pkgid, rrev, prev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.b, pkgRevKey(repo, name, version, ns, ch, pkgid, rrev))
	if err != nil {
		return false
	}
	for _, r := range revs {
		if r.Revision == prev {
			return true
		}
	}
	return false
}

// ─── package files ────────────────────────────────────────────────────────────

func (s *Storage) ListPackageFiles(repo, name, version, ns, ch, pkgid, rrev, prev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listFilesUnderPrefix(pkgRevDirPrefix(repo, name, version, ns, ch, pkgid, rrev, prev))
}

func (s *Storage) GetPackageFile(repo, name, version, ns, ch, pkgid, rrev, prev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, err := safeKey(pkgRevDirPrefix(repo, name, version, ns, ch, pkgid, rrev, prev) + filename)
	if err != nil {
		return nil, 0, err
	}
	return s.b.GetStream(key)
}

func (s *Storage) PutPackageFile(repo, name, version, ns, ch, pkgid, rrev, prev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, err := safeKey(pkgRevDirPrefix(repo, name, version, ns, ch, pkgid, rrev, prev) + filename)
	if err != nil {
		return err
	}
	return s.b.PutStream(key, r, -1)
}

// ─── search ───────────────────────────────────────────────────────────────────

func (s *Storage) Search(repo, query string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.b.List(repo + "/")
	if err != nil {
		return nil, nil
	}

	seen := make(map[string]bool)
	var results []string

	for _, key := range keys {
		// key: {repo}/{name}/{version}/{ns}/{ch}/...
		rel := strings.TrimPrefix(key, repo+"/")
		parts := strings.SplitN(rel, "/", 6)
		if len(parts) < 5 {
			continue
		}
		name, version, ns, ch := parts[0], parts[1], parts[2], parts[3]
		if name == "" || strings.HasPrefix(name, "_") {
			continue
		}
		ref := fmt.Sprintf("%s/%s@%s/%s", name, version, ns, ch)
		if !seen[ref] && matchQuery(query, ref, name, version) {
			seen[ref] = true
			results = append(results, ref)
		}
	}
	sort.Strings(results)
	return results, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// listFilesUnderPrefix returns only the direct children (non-recursive) of prefix.
func (s *Storage) listFilesUnderPrefix(prefix string) (map[string]string, error) {
	keys, err := s.b.List(prefix)
	if err != nil {
		return map[string]string{}, nil
	}
	files := make(map[string]string)
	for _, key := range keys {
		base := strings.TrimPrefix(key, prefix)
		if base == "" || strings.Contains(base, "/") {
			continue
		}
		files[base] = ""
	}
	return files, nil
}

func matchQuery(query, ref, name, version string) bool {
	if query == "" || query == "*" {
		return true
	}
	pattern := strings.ToLower(query)
	return globMatch(pattern, strings.ToLower(ref)) ||
		globMatch(pattern, strings.ToLower(name)) ||
		globMatch(pattern, strings.ToLower(name+"/"+version))
}

func globMatch(pattern, s string) bool {
	if !strings.Contains(pattern, "*") {
		return s == pattern
	}
	parts := strings.Split(pattern, "*")
	idx := 0
	for i, p := range parts {
		if p == "" {
			continue
		}
		j := strings.Index(s[idx:], p)
		if j < 0 {
			return false
		}
		if i == 0 && j != 0 {
			return false
		}
		idx += j + len(p)
	}
	if !strings.HasSuffix(pattern, "*") {
		return idx == len(s)
	}
	return true
}

// unused keys kept for reference
var _ = recipeFileKey
var _ = pkgFileKey
