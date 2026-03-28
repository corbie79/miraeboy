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

// ContextRecord represents a dynamically-created context (stored in _contexts/contexts.json).
type ContextRecord struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	AnonymousAccess string    `json:"anonymous_access"`
	CreatedAt       time.Time `json:"created_at"`
}

// Storage manages all package files on the local filesystem.
//
// Directory layout:
//
//	{base}/
//	  _contexts/
//	    contexts.json                ← dynamically created contexts
//	  {context}/
//	    {name}/{version}/{username}/{channel}/
//	      recipe_revisions.json      ← []Revision
//	      {rrev}/
//	        conanfile.py
//	        conanmanifest.txt
//	        ...
//	      packages/
//	        {pkgid}/
//	          {rrev}/
//	            pkg_revisions.json   ← []Revision
//	            {prev}/
//	              conaninfo.txt
//	              conanmanifest.txt
//	              conan_package.tgz
//	              ...
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

// ─── paths ────────────────────────────────────────────────────────────────────

func (s *Storage) refDir(context, name, version, username, channel string) string {
	return filepath.Join(s.base, context, name, version, username, channel)
}

func (s *Storage) recipeRevFile(context, name, version, username, channel string) string {
	return filepath.Join(s.refDir(context, name, version, username, channel), "recipe_revisions.json")
}

func (s *Storage) recipeFilesDir(context, name, version, username, channel, rrev string) string {
	return filepath.Join(s.refDir(context, name, version, username, channel), rrev)
}

func (s *Storage) pkgRevFile(context, name, version, username, channel, pkgid, rrev string) string {
	return filepath.Join(s.refDir(context, name, version, username, channel), "packages", pkgid, rrev, "pkg_revisions.json")
}

func (s *Storage) pkgFilesDir(context, name, version, username, channel, pkgid, rrev, prev string) string {
	return filepath.Join(s.refDir(context, name, version, username, channel), "packages", pkgid, rrev, prev)
}

// ─── context registry ─────────────────────────────────────────────────────────

func (s *Storage) contextsFile() string {
	return filepath.Join(s.base, "_contexts", "contexts.json")
}

// ContextExists checks whether a context directory exists OR is registered dynamically.
func (s *Storage) ContextExists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Check if context directory exists in storage
	if _, err := os.Stat(filepath.Join(s.base, name)); err == nil {
		return true
	}
	// Check dynamic registry
	recs, err := s.readContextRegistry()
	if err != nil {
		return false
	}
	for _, r := range recs {
		if r.Name == name {
			return true
		}
	}
	return false
}

func (s *Storage) ListDynamicContexts() ([]ContextRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readContextRegistry()
}

func (s *Storage) AddDynamicContext(name, description, anonymousAccess string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs, err := s.readContextRegistry()
	if err != nil {
		return err
	}
	for _, r := range recs {
		if r.Name == name {
			return fmt.Errorf("context %q already exists", name)
		}
	}
	recs = append(recs, ContextRecord{
		Name:            name,
		Description:     description,
		AnonymousAccess: anonymousAccess,
		CreatedAt:       time.Now().UTC(),
	})

	// Ensure the context directory exists
	if err := os.MkdirAll(filepath.Join(s.base, name), 0755); err != nil {
		return err
	}
	return s.writeContextRegistry(recs)
}

func (s *Storage) DeleteDynamicContext(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	recs, err := s.readContextRegistry()
	if err != nil {
		return err
	}
	filtered := recs[:0]
	for _, r := range recs {
		if r.Name != name {
			filtered = append(filtered, r)
		}
	}
	if err := s.writeContextRegistry(filtered); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.base, name))
}

func (s *Storage) readContextRegistry() ([]ContextRecord, error) {
	data, err := os.ReadFile(s.contextsFile())
	if os.IsNotExist(err) {
		return []ContextRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var recs []ContextRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

func (s *Storage) writeContextRegistry(recs []ContextRecord) error {
	dir := filepath.Dir(s.contextsFile())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.contextsFile(), data, 0644)
}

// ─── recipe revisions ─────────────────────────────────────────────────────────

func (s *Storage) GetRecipeRevisions(context, name, version, username, channel string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.recipeRevFile(context, name, version, username, channel))
}

func (s *Storage) AddRecipeRevision(context, name, version, username, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.refDir(context, name, version, username, channel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendRevision(s.recipeRevFile(context, name, version, username, channel), rrev)
}

func (s *Storage) DeleteRecipeRevision(context, name, version, username, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.recipeFilesDir(context, name, version, username, channel, rrev)); err != nil {
		return err
	}
	return removeRevision(s.recipeRevFile(context, name, version, username, channel), rrev)
}

func (s *Storage) RecipeRevisionExists(context, name, version, username, channel, rrev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.recipeRevFile(context, name, version, username, channel))
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

func (s *Storage) ListRecipeFiles(context, name, version, username, channel, rrev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.recipeFilesDir(context, name, version, username, channel, rrev))
}

func (s *Storage) GetRecipeFile(context, name, version, username, channel, rrev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.recipeFilesDir(context, name, version, username, channel, rrev), filename)
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

func (s *Storage) PutRecipeFile(context, name, version, username, channel, rrev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.recipeFilesDir(context, name, version, username, channel, rrev)
	return writeFile(dir, filename, r)
}

// ─── package revisions ────────────────────────────────────────────────────────

func (s *Storage) GetPackageRevisions(context, name, version, username, channel, pkgid, rrev string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.pkgRevFile(context, name, version, username, channel, pkgid, rrev))
}

func (s *Storage) AddPackageRevision(context, name, version, username, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.pkgRevFile(context, name, version, username, channel, pkgid, rrev))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendRevision(s.pkgRevFile(context, name, version, username, channel, pkgid, rrev), prev)
}

func (s *Storage) DeletePackageRevision(context, name, version, username, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.pkgFilesDir(context, name, version, username, channel, pkgid, rrev, prev)); err != nil {
		return err
	}
	return removeRevision(s.pkgRevFile(context, name, version, username, channel, pkgid, rrev), prev)
}

func (s *Storage) PackageRevisionExists(context, name, version, username, channel, pkgid, rrev, prev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.pkgRevFile(context, name, version, username, channel, pkgid, rrev))
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

func (s *Storage) ListPackageFiles(context, name, version, username, channel, pkgid, rrev, prev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.pkgFilesDir(context, name, version, username, channel, pkgid, rrev, prev))
}

func (s *Storage) GetPackageFile(context, name, version, username, channel, pkgid, rrev, prev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.pkgFilesDir(context, name, version, username, channel, pkgid, rrev, prev), filename)
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

func (s *Storage) PutPackageFile(context, name, version, username, channel, pkgid, rrev, prev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.pkgFilesDir(context, name, version, username, channel, pkgid, rrev, prev)
	return writeFile(dir, filename, r)
}

// ─── search ───────────────────────────────────────────────────────────────────

// Search returns package references matching a glob-style query within a context.
func (s *Storage) Search(context, query string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	contextDir := filepath.Join(s.base, context)
	var results []string

	names, err := os.ReadDir(contextDir)
	if err != nil {
		return nil, nil
	}

	for _, nameEntry := range names {
		if !nameEntry.IsDir() || strings.HasPrefix(nameEntry.Name(), "_") {
			continue
		}
		name := nameEntry.Name()
		versions, _ := os.ReadDir(filepath.Join(contextDir, name))
		for _, vEntry := range versions {
			if !vEntry.IsDir() {
				continue
			}
			version := vEntry.Name()
			users, _ := os.ReadDir(filepath.Join(contextDir, name, version))
			for _, uEntry := range users {
				if !uEntry.IsDir() {
					continue
				}
				username := uEntry.Name()
				channels, _ := os.ReadDir(filepath.Join(contextDir, name, version, username))
				for _, cEntry := range channels {
					if !cEntry.IsDir() {
						continue
					}
					channel := cEntry.Name()
					ref := fmt.Sprintf("%s/%s@%s/%s", name, version, username, channel)
					if matchQuery(query, ref, name, version, username, channel) {
						results = append(results, ref)
					}
				}
			}
		}
	}

	sort.Strings(results)
	return results, nil
}

// matchQuery checks if a reference matches the Conan search query.
func matchQuery(query, ref, name, version, username, channel string) bool {
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

// safeJoin joins dir and filename while preventing path traversal attacks.
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
