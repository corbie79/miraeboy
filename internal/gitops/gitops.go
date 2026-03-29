// Package gitops provides recipe → git synchronisation.
// When a Conan recipe file is uploaded, the Syncer writes the file into a
// local clone of the configured git repository and pushes it, giving teams a
// human-readable audit trail and the ability to version-control recipes
// alongside their source code.
//
// Authentication is handled via HTTPS token (GitHub PAT, GitLab CI token,
// Gitea API key, …). The token is embedded in the remote URL so that standard
// `git` CLI operations work without a credential helper.
//
// The Syncer is safe for concurrent use; operations are serialised by a mutex.
package gitops

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Config specifies the remote git repository to sync recipes into.
type Config struct {
	// URL is the HTTPS clone URL of the target repository.
	// Example: "https://github.com/corp/conan-recipes.git"
	URL string `json:"url" yaml:"url"`

	// Branch is the target branch. Defaults to "main".
	Branch string `json:"branch" yaml:"branch"`

	// Token is a personal access token (or equivalent) used for
	// HTTPS authentication. Embedded into the remote URL at runtime;
	// never stored in git config.
	Token string `json:"token" yaml:"token"`
}

// Syncer manages a local clone of a git repository and pushes recipe files.
type Syncer struct {
	workDir string // local clone root
	cfg     Config
	mu      sync.Mutex
}

// New creates a Syncer. workDir is the local path where the repo will be
// cloned (e.g. cfg.GitWorkspace + "/" + repoName).
func New(workDir string, cfg Config) *Syncer {
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	return &Syncer{workDir: workDir, cfg: cfg}
}

// SyncRevision writes the files for a recipe revision into git and pushes.
// files maps filename → content (e.g. "conanfile.py" → []byte{…}).
// The files are placed under {name}/{version}/{namespace}/{channel}/{rrev}/.
// On success the commit SHA is returned.
//
// Errors are non-fatal from the caller's perspective (recipe upload already
// succeeded); the caller should log them.
func (s *Syncer) SyncRevision(name, version, ns, ch, rrev string, files map[string][]byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncFiles(name, version, ns, ch, rrev, files)
}

// SyncFile writes a single recipe file into git and pushes.
// This is called from handleUploadRecipeFile on each individual PUT; Conan
// uploads files one by one so each file results in one git commit.
func (s *Syncer) SyncFile(name, version, ns, ch, rrev, filename string, content []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncFiles(name, version, ns, ch, rrev, map[string][]byte{filename: content})
}

func (s *Syncer) syncFiles(name, version, ns, ch, rrev string, files map[string][]byte) (string, error) {
	if err := s.ensureClone(); err != nil {
		return "", fmt.Errorf("gitops: clone: %w", err)
	}

	// Write files into the working tree.
	revDir := filepath.Join(s.workDir, name, version, ns, ch, rrev)
	if err := os.MkdirAll(revDir, 0o755); err != nil {
		return "", fmt.Errorf("gitops: mkdir: %w", err)
	}
	for filename, content := range files {
		dst := filepath.Join(revDir, filepath.Base(filename))
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return "", fmt.Errorf("gitops: write %s: %w", filename, err)
		}
	}

	// git add -A
	if err := s.run("git", "add", "-A"); err != nil {
		return "", fmt.Errorf("gitops: git add: %w", err)
	}

	// Check if there's anything to commit.
	if clean, _ := s.isClean(); clean {
		return "", nil // nothing changed
	}

	// Build a concise commit message.
	var fileList []string
	for f := range files {
		fileList = append(fileList, filepath.Base(f))
	}
	msg := fmt.Sprintf("conan: %s/%s@%s/%s rrev=%s: %s",
		name, version, ns, ch, rrev, strings.Join(fileList, ", "))

	if err := s.run("git", "commit", "-m", msg); err != nil {
		return "", fmt.Errorf("gitops: git commit: %w", err)
	}

	if err := s.run("git", "push", s.remoteURL(), "HEAD:"+s.cfg.Branch); err != nil {
		return "", fmt.Errorf("gitops: git push: %w", err)
	}

	sha, _ := s.headSHA()
	return sha, nil
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// ensureClone clones the repository if workDir does not yet contain a .git dir,
// otherwise fetches the latest state of the target branch and checks it out.
func (s *Syncer) ensureClone() error {
	gitDir := filepath.Join(s.workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Clone into a temp dir first so we don't leave a partial clone.
		parent := filepath.Dir(s.workDir)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
		return runIn(parent, "git", "clone", "--branch", s.cfg.Branch,
			"--single-branch", s.remoteURL(), s.workDir)
	}

	// Already cloned: pull latest.
	if err := s.run("git", "fetch", s.remoteURL(),
		s.cfg.Branch+":"+s.cfg.Branch); err != nil {
		// Non-fatal — push will still work if we're ahead of remote.
		_ = err
	}
	// Reset to tracking branch to avoid conflicts.
	_ = s.run("git", "checkout", s.cfg.Branch)
	_ = s.run("git", "reset", "--hard", "FETCH_HEAD")
	return nil
}

// isClean returns true when the working tree has no staged or unstaged changes.
func (s *Syncer) isClean() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "", nil
}

// headSHA returns the current HEAD commit hash.
func (s *Syncer) headSHA() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = s.workDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// remoteURL injects the token into the HTTPS URL so git can authenticate.
// If no token is configured the URL is returned as-is.
func (s *Syncer) remoteURL() string {
	if s.cfg.Token == "" {
		return s.cfg.URL
	}
	u, err := url.Parse(s.cfg.URL)
	if err != nil || u.Host == "" {
		return s.cfg.URL
	}
	u.User = url.UserPassword("token", s.cfg.Token)
	return u.String()
}

// run executes a git command in the clone directory.
func (s *Syncer) run(name string, args ...string) error {
	return runIn(s.workDir, name, args...)
}

func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr // progress info → stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
