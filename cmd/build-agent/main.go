// build-agent: 빌드 에이전트
// 각 플랫폼(Windows/Mac/Linux) 머신에서 실행하여 빌드 서버로부터 작업을 받아 네이티브 빌드 후 업로드
//
// 사용법:
//   ./build-agent --server=http://build-server:8500 --api-key=secret
//   ./build-agent --server=http://build-server:8500 --api-key=secret --workspace=C:\build
//
// 필요 환경:
//   - Go 1.22+
//   - Node.js + npm (프론트엔드 빌드용)
//   - git
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Version is set by -ldflags at build time.
var Version = "dev"

// ─── Model ────────────────────────────────────────────────────────────────────

type Job struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	RepoURL  string `json:"repo_url"`
	Ref      string `json:"ref"`
	Version  string `json:"version"`
}

// ─── Agent ────────────────────────────────────────────────────────────────────

type Agent struct {
	id        string
	serverURL string
	apiKey    string
	workspace string
	platform  string // "linux/amd64", "darwin/arm64", "windows/amd64" …
}

// poll asks the build server for the next pending job matching this agent's platform.
// Returns nil, nil when no job is available (HTTP 204).
func (a *Agent) poll() (*Job, error) {
	body, _ := json.Marshal(map[string]string{
		"agent_id": a.id,
		"platform": a.platform,
	})
	req, _ := http.NewRequest(http.MethodPost, a.serverURL+"/api/agent/poll", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("poll: status %d: %s", resp.StatusCode, b)
	}

	var job Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("poll decode: %w", err)
	}
	return &job, nil
}

// run executes the full build pipeline for a job and reports the result.
func (a *Agent) run(job *Job) {
	log.Printf("[job %s] start: %s@%s version=%s platform=%s", job.ID, job.RepoURL, job.Ref, job.Version, job.Platform)

	artifactPath, err := a.build(job)
	status, errMsg := "success", ""
	if err != nil {
		status, errMsg = "failed", err.Error()
		log.Printf("[job %s] FAILED: %v", job.ID, err)
	} else {
		log.Printf("[job %s] SUCCESS: %s", job.ID, artifactPath)
	}

	if uploadErr := a.done(job.ID, status, errMsg, artifactPath); uploadErr != nil {
		log.Printf("[job %s] upload error: %v", job.ID, uploadErr)
	}

	// Clean up artifact file after upload
	if artifactPath != "" {
		os.Remove(artifactPath)
	}
}

// build clones/updates the repo, builds the frontend + binary, and returns the archive path.
func (a *Agent) build(job *Job) (string, error) {
	repoDir := filepath.Join(a.workspace, repoSlug(job.RepoURL))

	// ── 1. Git clone / fetch ──────────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		log.Printf("[job %s] git clone %s", job.ID, job.RepoURL)
		if err := runIn(a.workspace, "git", "clone", job.RepoURL, repoDir); err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
	} else {
		log.Printf("[job %s] git fetch", job.ID)
		runIn(repoDir, "git", "fetch", "--all", "--tags") // non-fatal
	}

	// Checkout target ref
	if err := runIn(repoDir, "git", "checkout", job.Ref); err != nil {
		if err2 := runIn(repoDir, "git", "checkout", "origin/"+job.Ref); err2 != nil {
			return "", fmt.Errorf("git checkout %s: %w", job.Ref, err)
		}
	}
	runIn(repoDir, "git", "pull", "--ff-only") // ignore: may be detached HEAD

	// ── 2. Frontend build ─────────────────────────────────────────────────────
	webDir := filepath.Join(repoDir, "web")
	if _, err := os.Stat(filepath.Join(webDir, "package.json")); err == nil {
		log.Printf("[job %s] npm install", job.ID)
		if err := runIn(webDir, npmBin(), "ci", "--silent"); err != nil {
			return "", fmt.Errorf("npm ci: %w", err)
		}
		log.Printf("[job %s] npm run build", job.ID)
		if err := runIn(webDir, npmBin(), "run", "build", "--silent"); err != nil {
			return "", fmt.Errorf("npm build: %w", err)
		}
	}

	// ── 3. Go build ───────────────────────────────────────────────────────────
	binName := "miraeboy"
	if runtime.GOOS == "windows" {
		binName = "miraeboy.exe"
	}
	binPath := filepath.Join(repoDir, binName)
	ldflags := fmt.Sprintf("-s -w -X main.Version=%s", job.Version)

	log.Printf("[job %s] go build", job.ID)
	if err := runIn(repoDir, "go", "build",
		"-ldflags="+ldflags,
		"-o", binPath,
		".",
	); err != nil {
		return "", fmt.Errorf("go build: %w", err)
	}

	// ── 4. Create archive ─────────────────────────────────────────────────────
	platform := strings.ReplaceAll(job.Platform, "/", "-") // "linux-amd64"
	archiveName := fmt.Sprintf("miraeboy-%s-%s.%s", job.Version, platform, archiveExt())
	archivePath := filepath.Join(a.workspace, archiveName)

	files := []string{binPath}
	if cfg := filepath.Join(repoDir, "config.yaml"); fileExists(cfg) {
		files = append(files, cfg)
	}

	log.Printf("[job %s] creating archive: %s", job.ID, archiveName)
	if err := createArchive(archivePath, files); err != nil {
		return "", fmt.Errorf("archive: %w", err)
	}
	return archivePath, nil
}

// done uploads the job result and artifact to the build server.
func (a *Agent) done(jobID, status, errMsg, artifactPath string) error {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("status", status)
	mw.WriteField("error", errMsg)

	if status == "success" && artifactPath != "" {
		f, err := os.Open(artifactPath)
		if err == nil {
			defer f.Close()
			part, _ := mw.CreateFormFile("artifact", filepath.Base(artifactPath))
			io.Copy(part, f)
		}
	}
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost,
		a.serverURL+"/api/agent/jobs/"+jobID+"/done",
		&body,
	)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-API-Key", a.apiKey)
	// No timeout — artifact upload may be large
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("done: status %d: %s", resp.StatusCode, b)
	}
	return nil
}

// ─── Archive helpers ──────────────────────────────────────────────────────────

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

func createArchive(dst string, files []string) error {
	if runtime.GOOS == "windows" {
		return createZip(dst, files)
	}
	return createTarGz(dst, files)
}

func createTarGz(dst string, files []string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for _, src := range files {
		if err := tarAdd(tw, src); err != nil {
			return err
		}
	}
	return nil
}

func tarAdd(tw *tar.Writer, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(src)
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func createZip(dst string, files []string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	for _, src := range files {
		if err := zipAdd(zw, src); err != nil {
			return err
		}
	}
	return nil
}

func zipAdd(zw *zip.Writer, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(src)
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// ─── Misc helpers ─────────────────────────────────────────────────────────────

func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func npmBin() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}

func repoSlug(rawURL string) string {
	r := strings.NewReplacer(
		"https://", "", "http://", "",
		"/", "_", ":", "_", ".", "_",
	)
	return r.Replace(rawURL)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	server    := flag.String("server", "http://localhost:8500", "build server URL")
	apiKey    := flag.String("api-key", "", "API key (required)")
	workspace := flag.String("workspace", "./workspace", "working directory for clones and artifacts")
	agentID   := flag.String("id", "", "agent ID (default: hostname-os-arch)")
	interval  := flag.Duration("interval", 5*time.Second, "polling interval")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("--api-key is required")
	}

	id := *agentID
	if id == "" {
		hostname, _ := os.Hostname()
		id = fmt.Sprintf("%s-%s-%s", hostname, runtime.GOOS, runtime.GOARCH)
	}

	os.MkdirAll(*workspace, 0o755)

	a := &Agent{
		id:        id,
		serverURL: strings.TrimRight(*server, "/"),
		apiKey:    *apiKey,
		workspace: *workspace,
		platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	log.Printf("Build agent v%s started", Version)
	log.Printf("  agent:     %s", a.id)
	log.Printf("  platform:  %s", a.platform)
	log.Printf("  server:    %s", a.serverURL)
	log.Printf("  workspace: %s", a.workspace)
	log.Printf("  interval:  %s", *interval)

	consecutiveErrors := 0
	for {
		job, err := a.poll()
		if err != nil {
			consecutiveErrors++
			backoff := time.Duration(consecutiveErrors) * *interval
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			log.Printf("poll error (%dx): %v — retry in %s", consecutiveErrors, err, backoff)
			time.Sleep(backoff)
			continue
		}
		consecutiveErrors = 0

		if job != nil {
			a.run(job)
		} else {
			time.Sleep(*interval)
		}
	}
}
