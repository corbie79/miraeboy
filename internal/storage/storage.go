package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// RepoRecord is the full definition of a Conan repository, stored as
// _repos/{name}.json (one file per repository).
type RepoRecord struct {
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	Owner             string      `json:"owner"`
	AllowedNamespaces []string    `json:"allowed_namespaces"` // enforced @namespace on upload (empty = any)
	AllowedChannels   []string    `json:"allowed_channels"`   // enforced channel on upload (empty = any)
	AnonymousAccess   string      `json:"anonymous_access"`   // "read", "write", "none"
	Source            string      `json:"source"`             // "config" or "api"
	CreatedAt         time.Time   `json:"created_at"`
	Members           []RepoMember `json:"members"`
}

// Storage manages all package files on the local filesystem.
//
// Directory layout:
//
//	{base}/
//	  _repos/
//	    {repo-name}.json      ← RepoRecord (one file per repository)
//	  {repo}/
//	    {name}/{version}/{namespace}/{channel}/
//	      recipe_revisions.json
//	      {rrev}/
//	        conanfile.py, conanmanifest.txt, ...
//	      packages/{pkgid}/{rrev}/
//	        pkg_revisions.json
//	        {prev}/
//	          conaninfo.txt, conanmanifest.txt, conan_package.tgz, ...
type Storage struct {
	base string
	mu   sync.RWMutex
}

func New(base string) (*Storage, error) {
	if err := os.MkdirAll(base, 0755); err != nil {
		return nil, fmt.Errorf("storage init: %w", err)
	}
	return &Storage{base: base}, nil
}

// ─── repository registry ──────────────────────────────────────────────────────

func (s *Storage) reposDir() string {
	return filepath.Join(s.base, "_repos")
}

func (s *Storage) repoFile(name string) string {
	return filepath.Join(s.reposDir(), name+".json")
}

// RepoExists returns true when a repository with the given name is registered.
func (s *Storage) RepoExists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(s.repoFile(name))
	return err == nil
}

// GetRepo returns the RepoRecord for name, or (nil, nil) if not found.
func (s *Storage) GetRepo(name string) (*RepoRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readRepoFile(name)
}

// ListRepos returns all registered repositories.
func (s *Storage) ListRepos() ([]RepoRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.reposDir())
	if os.IsNotExist(err) {
		return []RepoRecord{}, nil
	}
	if err != nil {
		return nil, err
	}

	var repos []RepoRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		r, err := s.readRepoFile(name)
		if err != nil || r == nil {
			continue
		}
		repos = append(repos, *r)
	}
	return repos, nil
}

// SaveRepo writes a RepoRecord to disk (create or overwrite).
func (s *Storage) SaveRepo(r RepoRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeRepoFile(r)
}

// SeedRepo saves r only if the repository does not already exist.
// Used for config.yaml bootstrapping — safe to call on every startup.
func (s *Storage) SeedRepo(r RepoRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.repoFile(r.Name)); err == nil {
		return nil // already exists
	}
	return s.writeRepoFile(r)
}

// DeleteRepo removes the repository registry entry and all its package data.
func (s *Storage) DeleteRepo(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.repoFile(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(filepath.Join(s.base, name))
}

// GetUserRepoPermissions returns a map of repoName → permissionString for
// all repositories where username is a member or the owner.
func (s *Storage) GetUserRepoPermissions(username string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.reposDir())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		r, err := s.readRepoFile(name)
		if err != nil || r == nil {
			continue
		}
		// Owner always gets "owner" permission
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

func (s *Storage) readRepoFile(name string) (*RepoRecord, error) {
	data, err := os.ReadFile(s.repoFile(name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r RepoRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Storage) writeRepoFile(r RepoRecord) error {
	if err := os.MkdirAll(s.reposDir(), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.base, r.Name), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.repoFile(r.Name), data, 0644)
}

// ─── paths ────────────────────────────────────────────────────────────────────

func (s *Storage) refDir(repo, name, version, namespace, channel string) string {
	return filepath.Join(s.base, repo, name, version, namespace, channel)
}

func (s *Storage) recipeRevFile(repo, name, version, namespace, channel string) string {
	return filepath.Join(s.refDir(repo, name, version, namespace, channel), "recipe_revisions.json")
}

func (s *Storage) recipeFilesDir(repo, name, version, namespace, channel, rrev string) string {
	return filepath.Join(s.refDir(repo, name, version, namespace, channel), rrev)
}

func (s *Storage) pkgRevFile(repo, name, version, namespace, channel, pkgid, rrev string) string {
	return filepath.Join(s.refDir(repo, name, version, namespace, channel), "packages", pkgid, rrev, "pkg_revisions.json")
}

func (s *Storage) pkgFilesDir(repo, name, version, namespace, channel, pkgid, rrev, prev string) string {
	return filepath.Join(s.refDir(repo, name, version, namespace, channel), "packages", pkgid, rrev, prev)
}

// ─── recipe revisions ─────────────────────────────────────────────────────────

func (s *Storage) GetRecipeRevisions(repo, name, version, namespace, channel string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.recipeRevFile(repo, name, version, namespace, channel))
}

func (s *Storage) AddRecipeRevision(repo, name, version, namespace, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.refDir(repo, name, version, namespace, channel), 0755); err != nil {
		return err
	}
	return appendRevision(s.recipeRevFile(repo, name, version, namespace, channel), rrev)
}

func (s *Storage) DeleteRecipeRevision(repo, name, version, namespace, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.recipeFilesDir(repo, name, version, namespace, channel, rrev)); err != nil {
		return err
	}
	return removeRevision(s.recipeRevFile(repo, name, version, namespace, channel), rrev)
}

func (s *Storage) RecipeRevisionExists(repo, name, version, namespace, channel, rrev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.recipeRevFile(repo, name, version, namespace, channel))
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

func (s *Storage) ListRecipeFiles(repo, name, version, namespace, channel, rrev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.recipeFilesDir(repo, name, version, namespace, channel, rrev))
}

func (s *Storage) GetRecipeFile(repo, name, version, namespace, channel, rrev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.recipeFilesDir(repo, name, version, namespace, channel, rrev), filename)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	info, _ := f.Stat()
	return f, info.Size(), nil
}

func (s *Storage) PutRecipeFile(repo, name, version, namespace, channel, rrev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeFile(s.recipeFilesDir(repo, name, version, namespace, channel, rrev), filename, r)
}

// ─── package revisions ────────────────────────────────────────────────────────

func (s *Storage) GetPackageRevisions(repo, name, version, namespace, channel, pkgid, rrev string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.pkgRevFile(repo, name, version, namespace, channel, pkgid, rrev))
}

func (s *Storage) AddPackageRevision(repo, name, version, namespace, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.pkgRevFile(repo, name, version, namespace, channel, pkgid, rrev))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendRevision(s.pkgRevFile(repo, name, version, namespace, channel, pkgid, rrev), prev)
}

func (s *Storage) DeletePackageRevision(repo, name, version, namespace, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.pkgFilesDir(repo, name, version, namespace, channel, pkgid, rrev, prev)); err != nil {
		return err
	}
	return removeRevision(s.pkgRevFile(repo, name, version, namespace, channel, pkgid, rrev), prev)
}

func (s *Storage) PackageRevisionExists(repo, name, version, namespace, channel, pkgid, rrev, prev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.pkgRevFile(repo, name, version, namespace, channel, pkgid, rrev))
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

func (s *Storage) ListPackageFiles(repo, name, version, namespace, channel, pkgid, rrev, prev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.pkgFilesDir(repo, name, version, namespace, channel, pkgid, rrev, prev))
}

func (s *Storage) GetPackageFile(repo, name, version, namespace, channel, pkgid, rrev, prev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.pkgFilesDir(repo, name, version, namespace, channel, pkgid, rrev, prev), filename)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	info, _ := f.Stat()
	return f, info.Size(), nil
}

func (s *Storage) PutPackageFile(repo, name, version, namespace, channel, pkgid, rrev, prev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeFile(s.pkgFilesDir(repo, name, version, namespace, channel, pkgid, rrev, prev), filename, r)
}

// ─── search ───────────────────────────────────────────────────────────────────

// Search returns package references matching a glob query within a repository.
func (s *Storage) Search(repo, query string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	repoDir := filepath.Join(s.base, repo)
	var results []string

	names, err := os.ReadDir(repoDir)
	if err != nil {
		return nil, nil
	}
	for _, nameEntry := range names {
		if !nameEntry.IsDir() || strings.HasPrefix(nameEntry.Name(), "_") {
			continue
		}
		pkgName := nameEntry.Name()
		versions, _ := os.ReadDir(filepath.Join(repoDir, pkgName))
		for _, vEntry := range versions {
			if !vEntry.IsDir() {
				continue
			}
			version := vEntry.Name()
			namespaces, _ := os.ReadDir(filepath.Join(repoDir, pkgName, version))
			for _, nsEntry := range namespaces {
				if !nsEntry.IsDir() {
					continue
				}
				namespace := nsEntry.Name()
				channels, _ := os.ReadDir(filepath.Join(repoDir, pkgName, version, namespace))
				for _, cEntry := range channels {
					if !cEntry.IsDir() {
						continue
					}
					channel := cEntry.Name()
					ref := fmt.Sprintf("%s/%s@%s/%s", pkgName, version, namespace, channel)
					if matchQuery(query, ref, pkgName, version, namespace, channel) {
						results = append(results, ref)
					}
				}
			}
		}
	}
	sort.Strings(results)
	return results, nil
}

func matchQuery(query, ref, name, version, namespace, channel string) bool {
	if query == "" || query == "*" {
		return true
	}
	pattern := strings.ToLower(query)
	target := strings.ToLower(ref)
	return globMatch(pattern, target) ||
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

// ─── helpers ──────────────────────────────────────────────────────────────────

func readRevisions(path string) ([]Revision, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Revision{}, nil
	}
	if err != nil {
		return nil, err
	}
	var revs []Revision
	if err := json.Unmarshal(data, &revs); err != nil {
		return nil, err
	}
	return revs, nil
}

func appendRevision(path, rev string) error {
	revs, err := readRevisions(path)
	if err != nil {
		return err
	}
	filtered := revs[:0]
	for _, r := range revs {
		if r.Revision != rev {
			filtered = append(filtered, r)
		}
	}
	filtered = append(filtered, Revision{Revision: rev, Time: time.Now().UTC()})
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Time.After(filtered[j].Time)
	})
	return writeJSON(path, filtered)
}

func removeRevision(path, rev string) error {
	revs, err := readRevisions(path)
	if err != nil {
		return err
	}
	filtered := revs[:0]
	for _, r := range revs {
		if r.Revision != rev {
			filtered = append(filtered, r)
		}
	}
	return writeJSON(path, filtered)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func listFiles(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files[e.Name()] = fmt.Sprintf("%d", info.Size())
	}
	return files, nil
}

// safeJoin joins dir and filename while preventing path traversal.
func safeJoin(dir, filename string) (string, error) {
	clean := filepath.Clean(filename)
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid filename: %s", filename)
	}
	path := filepath.Join(dir, clean)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return "", fmt.Errorf("path traversal detected")
	}
	return path, nil
}

func writeFile(dir, filename string, r io.Reader) error {
	path, err := safeJoin(dir, filename)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}
