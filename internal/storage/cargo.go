package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ─── Cargo index entry ────────────────────────────────────────────────────────

// CargoIndexEntry is one line in a crate's sparse index file.
// Matches the format documented at https://doc.rust-lang.org/cargo/reference/registry-index.html
type CargoIndexEntry struct {
	Name    string            `json:"name"`
	Vers    string            `json:"vers"`
	Deps    []CargoDep        `json:"deps"`
	Cksum   string            `json:"cksum"` // SHA-256 hex of .crate file
	Feats   map[string][]string `json:"features"`
	Yanked  bool              `json:"yanked"`
	Links   *string           `json:"links"`
	V       int               `json:"v,omitempty"`    // format version (default 1)
}

type CargoDep struct {
	Name       string   `json:"name"`
	Req        string   `json:"req"`
	Features   []string `json:"features"`
	Optional   bool     `json:"optional"`
	DefaultFeatures bool `json:"default_features"`
	Target     *string  `json:"target"`
	Kind       string   `json:"kind"` // "normal", "dev", "build"
	Registry   *string  `json:"registry"`
	Package    *string  `json:"package"`
}

// ─── key helpers ─────────────────────────────────────────────────────────────

// cargoIndexKey returns the sparse index path for a crate name.
// Mirrors crates.io convention: 1/{n}, 2/{n}, 3/{c}/{n}, {ab}/{cd}/{n}
func CargoIndexPrefix(name string) string {
	lower := strings.ToLower(name)
	switch len(lower) {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3/" + string(lower[0])
	default:
		return lower[0:2] + "/" + lower[2:4]
	}
}

func cargoIndexKey(repo, name string) string {
	return fmt.Sprintf("_cargo/%s/index/%s/%s", repo, CargoIndexPrefix(name), strings.ToLower(name))
}

func cargoFileKey(repo, name, version string) string {
	return fmt.Sprintf("_cargo/%s/crates/%s/%s/%s-%s.crate", repo, strings.ToLower(name), version, strings.ToLower(name), version)
}

func cargoSearchPrefix(repo string) string {
	return fmt.Sprintf("_cargo/%s/index/", repo)
}

// ─── storage methods ──────────────────────────────────────────────────────────

// GetCargoIndex returns all index entries for a crate (one per version).
func (s *Storage) GetCargoIndex(repo, name string) ([]CargoIndexEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.b.Get(cargoIndexKey(repo, name))
	if err != nil {
		return nil, nil // not found → empty
	}
	var entries []CargoIndexEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e CargoIndexEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// AppendCargoIndex adds or updates a version entry in the crate index.
func (s *Storage) AppendCargoIndex(repo string, entry CargoIndexEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := cargoIndexKey(repo, entry.Name)
	existing, _ := s.b.Get(key)

	// Replace existing version line if present, otherwise append.
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(existing)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e CargoIndexEntry
		if json.Unmarshal([]byte(line), &e) == nil && e.Vers == entry.Vers {
			continue // will be replaced
		}
		lines = append(lines, line)
	}
	newLine, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	lines = append(lines, string(newLine))
	return s.b.Put(key, []byte(strings.Join(lines, "\n")+"\n"))
}

// SetCargoYanked sets the yanked flag for a specific crate version.
func (s *Storage) SetCargoYanked(repo, name, version string, yanked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := cargoIndexKey(repo, name)
	data, err := s.b.Get(key)
	if err != nil {
		return fmt.Errorf("crate %s not found", name)
	}

	var lines []string
	found := false
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e CargoIndexEntry
		if json.Unmarshal([]byte(line), &e) == nil && e.Vers == version {
			e.Yanked = yanked
			updated, _ := json.Marshal(e)
			lines = append(lines, string(updated))
			found = true
			continue
		}
		lines = append(lines, line)
	}
	if !found {
		return fmt.Errorf("version %s not found for crate %s", version, name)
	}
	return s.b.Put(key, []byte(strings.Join(lines, "\n")+"\n"))
}

// PutCrateFile stores a .crate binary and returns its SHA-256 checksum.
func (s *Storage) PutCrateFile(repo, name, version string, r io.Reader) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	cksum := fmt.Sprintf("%x", sum)

	key := cargoFileKey(repo, name, version)
	if err := s.b.Put(key, data); err != nil {
		return "", err
	}
	return cksum, nil
}

// GetCrateFile streams a .crate binary.
func (s *Storage) GetCrateFile(repo, name, version string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.b.GetStream(cargoFileKey(repo, name, version))
}

// SearchCargo returns crate names matching query across all index entries.
func (s *Storage) SearchCargo(repo, query string) ([]CargoIndexEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.b.List(cargoSearchPrefix(repo))
	if err != nil {
		return nil, nil
	}

	q := strings.ToLower(query)
	seen := make(map[string]CargoIndexEntry) // name → latest non-yanked version

	for _, key := range keys {
		// skip sub-directories (only leaf files contain index entries)
		rel := strings.TrimPrefix(key, cargoSearchPrefix(repo))
		if strings.Contains(rel, "/") {
			// depth > 1: it's a file under a prefix dir — still valid
		}
		data, err := s.b.Get(key)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var e CargoIndexEntry
			if json.Unmarshal([]byte(line), &e) != nil || e.Yanked {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(e.Name), q) {
				continue
			}
			// keep latest version per crate
			if prev, ok := seen[e.Name]; !ok || versionGt(e.Vers, prev.Vers) {
				seen[e.Name] = e
			}
		}
	}

	results := make([]CargoIndexEntry, 0, len(seen))
	for _, e := range seen {
		results = append(results, e)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

// versionGt is a naive semver comparison (good enough for "latest").
func versionGt(a, b string) bool {
	return a > b // lexicographic works for well-formed semver in most cases
}
