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

// Revision represents a single revision entry stored in revisions.json
type Revision struct {
	Revision string    `json:"revision"`
	Time     time.Time `json:"time"`
}

// Storage manages all package files on the local filesystem.
//
// Directory layout:
//
//	{base}/{name}/{version}/{username}/{channel}/
//	  recipe_revisions.json           ← []Revision
//	  {rrev}/
//	    conanfile.py
//	    conanmanifest.txt
//	    ...
//	  packages/
//	    {pkgid}/
//	      {rrev}/
//	        pkg_revisions.json         ← []Revision
//	        {prev}/
//	          conaninfo.txt
//	          conanmanifest.txt
//	          conan_package.tgz
//	          ...
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

func (s *Storage) refDir(name, version, username, channel string) string {
	return filepath.Join(s.base, name, version, username, channel)
}

func (s *Storage) recipeRevFile(name, version, username, channel string) string {
	return filepath.Join(s.refDir(name, version, username, channel), "recipe_revisions.json")
}

func (s *Storage) recipeFilesDir(name, version, username, channel, rrev string) string {
	return filepath.Join(s.refDir(name, version, username, channel), rrev)
}

func (s *Storage) pkgRevFile(name, version, username, channel, pkgid, rrev string) string {
	return filepath.Join(s.refDir(name, version, username, channel), "packages", pkgid, rrev, "pkg_revisions.json")
}

func (s *Storage) pkgFilesDir(name, version, username, channel, pkgid, rrev, prev string) string {
	return filepath.Join(s.refDir(name, version, username, channel), "packages", pkgid, rrev, prev)
}

// ─── recipe revisions ─────────────────────────────────────────────────────────

func (s *Storage) GetRecipeRevisions(name, version, username, channel string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.recipeRevFile(name, version, username, channel))
}

func (s *Storage) AddRecipeRevision(name, version, username, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.refDir(name, version, username, channel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendRevision(s.recipeRevFile(name, version, username, channel), rrev)
}

func (s *Storage) DeleteRecipeRevision(name, version, username, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.recipeFilesDir(name, version, username, channel, rrev)); err != nil {
		return err
	}
	return removeRevision(s.recipeRevFile(name, version, username, channel), rrev)
}

func (s *Storage) RecipeRevisionExists(name, version, username, channel, rrev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.recipeRevFile(name, version, username, channel))
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

func (s *Storage) ListRecipeFiles(name, version, username, channel, rrev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.recipeFilesDir(name, version, username, channel, rrev))
}

func (s *Storage) GetRecipeFile(name, version, username, channel, rrev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.recipeFilesDir(name, version, username, channel, rrev), filename)
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

func (s *Storage) PutRecipeFile(name, version, username, channel, rrev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.recipeFilesDir(name, version, username, channel, rrev)
	return writeFile(dir, filename, r)
}

// ─── package revisions ────────────────────────────────────────────────────────

func (s *Storage) GetPackageRevisions(name, version, username, channel, pkgid, rrev string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.pkgRevFile(name, version, username, channel, pkgid, rrev))
}

func (s *Storage) AddPackageRevision(name, version, username, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.pkgRevFile(name, version, username, channel, pkgid, rrev))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendRevision(s.pkgRevFile(name, version, username, channel, pkgid, rrev), prev)
}

func (s *Storage) DeletePackageRevision(name, version, username, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.pkgFilesDir(name, version, username, channel, pkgid, rrev, prev)); err != nil {
		return err
	}
	return removeRevision(s.pkgRevFile(name, version, username, channel, pkgid, rrev), prev)
}

func (s *Storage) PackageRevisionExists(name, version, username, channel, pkgid, rrev, prev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.pkgRevFile(name, version, username, channel, pkgid, rrev))
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

func (s *Storage) ListPackageFiles(name, version, username, channel, pkgid, rrev, prev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.pkgFilesDir(name, version, username, channel, pkgid, rrev, prev))
}

func (s *Storage) GetPackageFile(name, version, username, channel, pkgid, rrev, prev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.pkgFilesDir(name, version, username, channel, pkgid, rrev, prev), filename)
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

func (s *Storage) PutPackageFile(name, version, username, channel, pkgid, rrev, prev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.pkgFilesDir(name, version, username, channel, pkgid, rrev, prev)
	return writeFile(dir, filename, r)
}

// ─── search ───────────────────────────────────────────────────────────────────

// Search returns package references matching a glob-style query.
// query format: "name*", "name/version@user/channel", etc.
func (s *Storage) Search(query string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []string

	entries, err := os.ReadDir(s.base)
	if err != nil {
		return nil, nil
	}

	for _, nameEntry := range entries {
		if !nameEntry.IsDir() {
			continue
		}
		name := nameEntry.Name()

		versionsDir := filepath.Join(s.base, name)
		versions, _ := os.ReadDir(versionsDir)
		for _, vEntry := range versions {
			if !vEntry.IsDir() {
				continue
			}
			version := vEntry.Name()

			usersDir := filepath.Join(versionsDir, version)
			users, _ := os.ReadDir(usersDir)
			for _, uEntry := range users {
				if !uEntry.IsDir() {
					continue
				}
				username := uEntry.Name()

				channelsDir := filepath.Join(usersDir, username)
				channels, _ := os.ReadDir(channelsDir)
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
// Supports "*" as wildcard and patterns like "name*", "name/version@*".
func matchQuery(query, ref, name, version, username, channel string) bool {
	if query == "" || query == "*" {
		return true
	}
	// Simple glob: replace * with a placeholder then match
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
	// Deduplicate: remove existing entry with same revision
	filtered := revs[:0]
	for _, r := range revs {
		if r.Revision != rev {
			filtered = append(filtered, r)
		}
	}
	filtered = append(filtered, Revision{Revision: rev, Time: time.Now().UTC()})
	// Latest first
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
	// Ensure the resolved path is still under dir
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
