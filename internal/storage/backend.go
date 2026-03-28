package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backend abstracts the underlying object store (filesystem or S3-compatible).
type Backend interface {
	// Get reads an object and returns its contents.
	Get(key string) ([]byte, error)
	// Put writes data to the given key.
	Put(key string, data []byte) error
	// GetStream opens a streaming reader for the given key.
	GetStream(key string) (io.ReadCloser, int64, error)
	// PutStream writes a stream to the given key.
	PutStream(key string, r io.Reader, size int64) error
	// Delete removes an object.
	Delete(key string) error
	// DeletePrefix removes all objects with the given prefix.
	DeletePrefix(prefix string) error
	// List returns all keys with the given prefix.
	List(prefix string) ([]string, error)
	// Exists checks whether a key exists.
	Exists(key string) bool
}

// ─── filesystem backend ───────────────────────────────────────────────────────

type fsBackend struct{ base string }

func NewFSBackend(base string) (Backend, error) {
	if err := os.MkdirAll(base, 0755); err != nil {
		return nil, fmt.Errorf("fs backend init: %w", err)
	}
	return &fsBackend{base: base}, nil
}

func (b *fsBackend) path(key string) string {
	return filepath.Join(b.base, filepath.FromSlash(key))
}

func (b *fsBackend) Get(key string) ([]byte, error) {
	data, err := os.ReadFile(b.path(key))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return data, err
}

func (b *fsBackend) Put(key string, data []byte) error {
	p := b.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func (b *fsBackend) GetStream(key string) (io.ReadCloser, int64, error) {
	f, err := os.Open(b.path(key))
	if os.IsNotExist(err) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (b *fsBackend) PutStream(key string, r io.Reader, _ int64) error {
	p := b.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (b *fsBackend) Delete(key string) error {
	err := os.Remove(b.path(key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (b *fsBackend) DeletePrefix(prefix string) error {
	return os.RemoveAll(b.path(prefix))
}

func (b *fsBackend) List(prefix string) ([]string, error) {
	root := b.path(prefix)
	var keys []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(b.base, p)
			keys = append(keys, filepath.ToSlash(rel))
		}
		return nil
	})
	return keys, err
}

func (b *fsBackend) Exists(key string) bool {
	_, err := os.Stat(b.path(key))
	return err == nil
}

// S3Config holds S3-compatible storage configuration.
// Populated from config.yaml; used by NewS3Backend when the s3 build tag is set.
type S3Config struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Region          string
}

// ErrNotFound is returned when a requested object does not exist.
var ErrNotFound = errors.New("not found")

// ─── JSON helpers shared by both backends ─────────────────────────────────────

func getJSON(b Backend, key string, v any) error {
	data, err := b.Get(key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil // treat missing as zero value
		}
		return err
	}
	return json.Unmarshal(data, v)
}

func putJSON(b Backend, key string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return b.Put(key, data)
}

// safeKey validates that a key segment doesn't escape the intended prefix.
func safeKey(parts ...string) (string, error) {
	for _, p := range parts {
		clean := filepath.Clean(p)
		if strings.Contains(clean, "..") || strings.ContainsAny(p, "\\") {
			return "", fmt.Errorf("invalid path segment: %q", p)
		}
	}
	return strings.Join(parts, "/"), nil
}

// ─── revision helpers ─────────────────────────────────────────────────────────

func readRevisions(b Backend, key string) ([]Revision, error) {
	var revs []Revision
	if err := getJSON(b, key, &revs); err != nil {
		return nil, err
	}
	if revs == nil {
		revs = []Revision{}
	}
	return revs, nil
}

func appendRevision(b Backend, key, rev string) error {
	revs, err := readRevisions(b, key)
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
	// newest first
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	return putJSON(b, key, filtered)
}

func removeRevision(b Backend, key, rev string) error {
	revs, err := readRevisions(b, key)
	if err != nil {
		return err
	}
	filtered := revs[:0]
	for _, r := range revs {
		if r.Revision != rev {
			filtered = append(filtered, r)
		}
	}
	return putJSON(b, key, filtered)
}
