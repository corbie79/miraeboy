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

// GroupMember holds a user's permission within a package group.
type GroupMember struct {
	Username   string `json:"username"`
	Permission string `json:"permission"` // "read", "write", "delete", "owner"
}

// GroupRecord is the full definition of a package group, stored as
// _groups/{name}.json (one file per group).
type GroupRecord struct {
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	Owner           string        `json:"owner"`
	ConanUser       string        `json:"conan_user"`    // enforced @user on upload ("" = any)
	ConanChannel    string        `json:"conan_channel"` // enforced @channel on upload ("" = any)
	AnonymousAccess string        `json:"anonymous_access"` // "read", "write", "none"
	Source          string        `json:"source"`           // "config" or "api"
	CreatedAt       time.Time     `json:"created_at"`
	Members         []GroupMember `json:"members"`
}

// Storage manages all package files on the local filesystem.
//
// Directory layout:
//
//	{base}/
//	  _groups/
//	    {group-name}.json     ← GroupRecord (one file per group)
//	  {group}/
//	    {name}/{version}/{username}/{channel}/
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

// ─── group registry ───────────────────────────────────────────────────────────

func (s *Storage) groupsDir() string {
	return filepath.Join(s.base, "_groups")
}

func (s *Storage) groupFile(name string) string {
	return filepath.Join(s.groupsDir(), name+".json")
}

// GroupExists returns true when a group with the given name is registered.
func (s *Storage) GroupExists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(s.groupFile(name))
	return err == nil
}

// GetGroup returns the GroupRecord for name, or (nil, nil) if not found.
func (s *Storage) GetGroup(name string) (*GroupRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readGroupFile(name)
}

// ListGroups returns all registered groups.
func (s *Storage) ListGroups() ([]GroupRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.groupsDir())
	if os.IsNotExist(err) {
		return []GroupRecord{}, nil
	}
	if err != nil {
		return nil, err
	}

	var groups []GroupRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		g, err := s.readGroupFile(name)
		if err != nil || g == nil {
			continue
		}
		groups = append(groups, *g)
	}
	return groups, nil
}

// SaveGroup writes a GroupRecord to disk (create or overwrite).
func (s *Storage) SaveGroup(g GroupRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeGroupFile(g)
}

// SeedGroup saves g only if the group does not already exist.
// Used for config.yaml bootstrapping — safe to call on every startup.
func (s *Storage) SeedGroup(g GroupRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.groupFile(g.Name)); err == nil {
		return nil // already exists
	}
	return s.writeGroupFile(g)
}

// DeleteGroup removes the group registry entry and all its package data.
func (s *Storage) DeleteGroup(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.groupFile(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(filepath.Join(s.base, name))
}

// GetUserGroupPermissions returns a map of groupName → permissionString for
// all groups where username is a member or the owner.
func (s *Storage) GetUserGroupPermissions(username string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.groupsDir())
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
		g, err := s.readGroupFile(name)
		if err != nil || g == nil {
			continue
		}
		// Owner always gets "owner" permission
		if g.Owner == username {
			result[g.Name] = "owner"
			continue
		}
		for _, m := range g.Members {
			if m.Username == username {
				result[g.Name] = m.Permission
				break
			}
		}
	}
	return result, nil
}

func (s *Storage) readGroupFile(name string) (*GroupRecord, error) {
	data, err := os.ReadFile(s.groupFile(name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var g GroupRecord
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Storage) writeGroupFile(g GroupRecord) error {
	if err := os.MkdirAll(s.groupsDir(), 0755); err != nil {
		return err
	}
	// Ensure package data directory exists
	if err := os.MkdirAll(filepath.Join(s.base, g.Name), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.groupFile(g.Name), data, 0644)
}

// ─── paths ────────────────────────────────────────────────────────────────────

func (s *Storage) refDir(group, name, version, username, channel string) string {
	return filepath.Join(s.base, group, name, version, username, channel)
}

func (s *Storage) recipeRevFile(group, name, version, username, channel string) string {
	return filepath.Join(s.refDir(group, name, version, username, channel), "recipe_revisions.json")
}

func (s *Storage) recipeFilesDir(group, name, version, username, channel, rrev string) string {
	return filepath.Join(s.refDir(group, name, version, username, channel), rrev)
}

func (s *Storage) pkgRevFile(group, name, version, username, channel, pkgid, rrev string) string {
	return filepath.Join(s.refDir(group, name, version, username, channel), "packages", pkgid, rrev, "pkg_revisions.json")
}

func (s *Storage) pkgFilesDir(group, name, version, username, channel, pkgid, rrev, prev string) string {
	return filepath.Join(s.refDir(group, name, version, username, channel), "packages", pkgid, rrev, prev)
}

// ─── recipe revisions ─────────────────────────────────────────────────────────

func (s *Storage) GetRecipeRevisions(group, name, version, username, channel string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.recipeRevFile(group, name, version, username, channel))
}

func (s *Storage) AddRecipeRevision(group, name, version, username, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.refDir(group, name, version, username, channel), 0755); err != nil {
		return err
	}
	return appendRevision(s.recipeRevFile(group, name, version, username, channel), rrev)
}

func (s *Storage) DeleteRecipeRevision(group, name, version, username, channel, rrev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.recipeFilesDir(group, name, version, username, channel, rrev)); err != nil {
		return err
	}
	return removeRevision(s.recipeRevFile(group, name, version, username, channel), rrev)
}

func (s *Storage) RecipeRevisionExists(group, name, version, username, channel, rrev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.recipeRevFile(group, name, version, username, channel))
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

func (s *Storage) ListRecipeFiles(group, name, version, username, channel, rrev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.recipeFilesDir(group, name, version, username, channel, rrev))
}

func (s *Storage) GetRecipeFile(group, name, version, username, channel, rrev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.recipeFilesDir(group, name, version, username, channel, rrev), filename)
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

func (s *Storage) PutRecipeFile(group, name, version, username, channel, rrev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeFile(s.recipeFilesDir(group, name, version, username, channel, rrev), filename, r)
}

// ─── package revisions ────────────────────────────────────────────────────────

func (s *Storage) GetPackageRevisions(group, name, version, username, channel, pkgid, rrev string) ([]Revision, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return readRevisions(s.pkgRevFile(group, name, version, username, channel, pkgid, rrev))
}

func (s *Storage) AddPackageRevision(group, name, version, username, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.pkgRevFile(group, name, version, username, channel, pkgid, rrev))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return appendRevision(s.pkgRevFile(group, name, version, username, channel, pkgid, rrev), prev)
}

func (s *Storage) DeletePackageRevision(group, name, version, username, channel, pkgid, rrev, prev string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.pkgFilesDir(group, name, version, username, channel, pkgid, rrev, prev)); err != nil {
		return err
	}
	return removeRevision(s.pkgRevFile(group, name, version, username, channel, pkgid, rrev), prev)
}

func (s *Storage) PackageRevisionExists(group, name, version, username, channel, pkgid, rrev, prev string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	revs, err := readRevisions(s.pkgRevFile(group, name, version, username, channel, pkgid, rrev))
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

func (s *Storage) ListPackageFiles(group, name, version, username, channel, pkgid, rrev, prev string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return listFiles(s.pkgFilesDir(group, name, version, username, channel, pkgid, rrev, prev))
}

func (s *Storage) GetPackageFile(group, name, version, username, channel, pkgid, rrev, prev, filename string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := safeJoin(s.pkgFilesDir(group, name, version, username, channel, pkgid, rrev, prev), filename)
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

func (s *Storage) PutPackageFile(group, name, version, username, channel, pkgid, rrev, prev, filename string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeFile(s.pkgFilesDir(group, name, version, username, channel, pkgid, rrev, prev), filename, r)
}

// ─── search ───────────────────────────────────────────────────────────────────

// Search returns package references matching a glob query within a group.
func (s *Storage) Search(group, query string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groupDir := filepath.Join(s.base, group)
	var results []string

	names, err := os.ReadDir(groupDir)
	if err != nil {
		return nil, nil
	}
	for _, nameEntry := range names {
		if !nameEntry.IsDir() || strings.HasPrefix(nameEntry.Name(), "_") {
			continue
		}
		pkgName := nameEntry.Name()
		versions, _ := os.ReadDir(filepath.Join(groupDir, pkgName))
		for _, vEntry := range versions {
			if !vEntry.IsDir() {
				continue
			}
			version := vEntry.Name()
			users, _ := os.ReadDir(filepath.Join(groupDir, pkgName, version))
			for _, uEntry := range users {
				if !uEntry.IsDir() {
					continue
				}
				conanUser := uEntry.Name()
				channels, _ := os.ReadDir(filepath.Join(groupDir, pkgName, version, conanUser))
				for _, cEntry := range channels {
					if !cEntry.IsDir() {
						continue
					}
					channel := cEntry.Name()
					ref := fmt.Sprintf("%s/%s@%s/%s", pkgName, version, conanUser, channel)
					if matchQuery(query, ref, pkgName, version, conanUser, channel) {
						results = append(results, ref)
					}
				}
			}
		}
	}
	sort.Strings(results)
	return results, nil
}

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
