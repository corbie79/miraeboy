// build-server: 빌드 코디네이터 서버
//
// 사용법:
//   ./build-server --api-key=secret
//   ./build-server --addr=:8500 --api-key=secret --artifacts=./artifacts
//
// 빌드 트리거:
//   curl -X POST http://localhost:8500/api/build \
//     -H "X-API-Key: secret" \
//     -H "Content-Type: application/json" \
//     -d '{"repo_url":"http://gitea/corbie79/miraeboy","ref":"main","version":"v1.0.0"}'
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ─── Model ────────────────────────────────────────────────────────────────────

type Job struct {
	ID         string     `json:"id"`
	BuildID    string     `json:"build_id"`
	Platform   string     `json:"platform"` // "linux/amd64"
	Status     string     `json:"status"`   // pending | running | success | failed
	AgentID    string     `json:"agent_id,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	Artifact   string     `json:"artifact,omitempty"` // uploaded filename
	// Embedded for agent convenience
	RepoURL string `json:"repo_url"`
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

type Build struct {
	ID        string    `json:"id"`
	RepoURL   string    `json:"repo_url"`
	Ref       string    `json:"ref"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
	Jobs      []*Job    `json:"jobs"`
}

func calcStatus(jobs []*Job) string {
	counts := map[string]int{}
	for _, j := range jobs {
		counts[j.Status]++
	}
	switch {
	case counts["failed"] > 0:
		return "failed"
	case counts["running"] > 0:
		return "running"
	case counts["pending"] > 0:
		return "pending"
	case len(jobs) > 0:
		return "success"
	default:
		return "pending"
	}
}

// ─── Store ────────────────────────────────────────────────────────────────────

type Store struct {
	mu        sync.Mutex
	builds    map[string]*Build
	jobs      map[string]*Job
	artifacts string
	apiKey    string
}

func newStore(artifacts, apiKey string) *Store {
	os.MkdirAll(artifacts, 0o755)
	return &Store{
		builds:    make(map[string]*Build),
		jobs:      make(map[string]*Job),
		artifacts: artifacts,
		apiKey:    apiKey,
	}
}

func (st *Store) checkAuth(r *http.Request) bool {
	k := r.Header.Get("X-API-Key")
	if k == "" {
		k = r.URL.Query().Get("api_key")
	}
	return k == st.apiKey
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

const defaultPlatforms = "linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64"

// POST /api/build — trigger a new build
func (st *Store) handleTrigger(w http.ResponseWriter, r *http.Request) {
	if !st.checkAuth(r) {
		jsonErr(w, 401, "unauthorized")
		return
	}
	var req struct {
		RepoURL   string   `json:"repo_url"`
		Ref       string   `json:"ref"`
		Version   string   `json:"version"`
		Platforms []string `json:"platforms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if req.RepoURL == "" {
		jsonErr(w, 400, "repo_url is required")
		return
	}
	if req.Ref == "" {
		req.Ref = "main"
	}
	if req.Version == "" {
		req.Version = "dev"
	}
	if len(req.Platforms) == 0 {
		req.Platforms = strings.Split(defaultPlatforms, ",")
	}

	b := &Build{
		ID:        newID(),
		RepoURL:   req.RepoURL,
		Ref:       req.Ref,
		Version:   req.Version,
		CreatedAt: time.Now(),
	}

	st.mu.Lock()
	for _, p := range req.Platforms {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		j := &Job{
			ID:      newID(),
			BuildID: b.ID,
			Platform: p,
			Status:  "pending",
			RepoURL: req.RepoURL,
			Ref:     req.Ref,
			Version: req.Version,
		}
		b.Jobs = append(b.Jobs, j)
		st.jobs[j.ID] = j
	}
	b.Status = calcStatus(b.Jobs)
	st.builds[b.ID] = b
	st.mu.Unlock()

	log.Printf("build %s: %s@%s version=%s jobs=%d", b.ID, req.RepoURL, req.Ref, req.Version, len(b.Jobs))
	writeJSON(w, 201, b)
}

// GET /api/builds — list all builds newest first
func (st *Store) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	st.mu.Lock()
	list := make([]*Build, 0, len(st.builds))
	for _, b := range st.builds {
		b.Status = calcStatus(b.Jobs)
		list = append(list, b)
	}
	st.mu.Unlock()
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	writeJSON(w, 200, list)
}

// GET /api/builds/{id} — get build detail
func (st *Store) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	st.mu.Lock()
	b := st.builds[r.PathValue("id")]
	if b != nil {
		b.Status = calcStatus(b.Jobs)
	}
	st.mu.Unlock()
	if b == nil {
		jsonErr(w, 404, "build not found")
		return
	}
	writeJSON(w, 200, b)
}

// GET /api/artifacts/{build}/{file} — download artifact
func (st *Store) handleArtifact(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if strings.ContainsAny(file, "/\\") || strings.Contains(file, "..") {
		jsonErr(w, 400, "invalid filename")
		return
	}
	path := filepath.Join(st.artifacts, r.PathValue("build"), file)
	f, err := os.Open(path)
	if err != nil {
		jsonErr(w, 404, "artifact not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", "attachment; filename="+file)
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, f)
}

// POST /api/agent/poll — agent polls for next pending job matching its platform
func (st *Store) handlePoll(w http.ResponseWriter, r *http.Request) {
	if !st.checkAuth(r) {
		jsonErr(w, 401, "unauthorized")
		return
	}
	var req struct {
		AgentID  string `json:"agent_id"`
		Platform string `json:"platform"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Platform == "" {
		jsonErr(w, 400, "platform required")
		return
	}

	st.mu.Lock()
	var found *Job
	for _, j := range st.jobs {
		if j.Status == "pending" && j.Platform == req.Platform {
			found = j
			break
		}
	}
	if found != nil {
		now := time.Now()
		found.Status = "running"
		found.AgentID = req.AgentID
		found.StartedAt = &now
	}
	st.mu.Unlock()

	if found == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	log.Printf("job %s → agent %s (%s)", found.ID, req.AgentID, req.Platform)
	writeJSON(w, 200, found)
}

// POST /api/agent/jobs/{id}/done — agent submits result + artifact
func (st *Store) handleDone(w http.ResponseWriter, r *http.Request) {
	if !st.checkAuth(r) {
		jsonErr(w, 401, "unauthorized")
		return
	}

	st.mu.Lock()
	job := st.jobs[r.PathValue("id")]
	st.mu.Unlock()
	if job == nil {
		jsonErr(w, 404, "job not found")
		return
	}

	if err := r.ParseMultipartForm(1 << 30); err != nil { // 1 GB
		jsonErr(w, 400, err.Error())
		return
	}

	status := r.FormValue("status") // "success" or "failed"
	errMsg := r.FormValue("error")
	now := time.Now()

	st.mu.Lock()
	job.Status = status
	job.Error = errMsg
	job.FinishedAt = &now
	st.mu.Unlock()

	if status == "success" {
		if f, h, err := r.FormFile("artifact"); err == nil {
			defer f.Close()
			dir := filepath.Join(st.artifacts, job.BuildID)
			os.MkdirAll(dir, 0o755)
			dst, err := os.Create(filepath.Join(dir, h.Filename))
			if err == nil {
				io.Copy(dst, f)
				dst.Close()
				st.mu.Lock()
				job.Artifact = h.Filename
				st.mu.Unlock()
				log.Printf("job %s: artifact saved: %s", job.ID, h.Filename)
			}
		}
	}

	log.Printf("job %s: %s (agent=%s)", job.ID, status, job.AgentID)
	w.WriteHeader(http.StatusNoContent)
}

// GET / — HTML dashboard
func (st *Store) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

// ─── HTML Dashboard ───────────────────────────────────────────────────────────

const dashboardHTML = `<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8">
<title>miraeboy build server</title>
<style>
*{box-sizing:border-box}
body{font-family:monospace;margin:0;background:#111;color:#e2e8f0}
header{background:#1e293b;padding:1rem 2rem;border-bottom:1px solid #334155}
header h1{margin:0;color:#4ade80;font-size:1.2rem}
main{padding:2rem}
h2{color:#94a3b8;font-size:0.9rem;text-transform:uppercase;letter-spacing:.1em;margin:2rem 0 .5rem}
.form-row{display:flex;gap:.5rem;flex-wrap:wrap;align-items:center;margin:.25rem 0}
input,select{background:#1e293b;color:#e2e8f0;border:1px solid #334155;padding:.375rem .75rem;border-radius:4px;font-family:monospace;font-size:.875rem}
input:focus{outline:none;border-color:#4ade80}
.btn{background:#4ade80;color:#111;border:none;padding:.375rem 1rem;border-radius:4px;cursor:pointer;font-weight:bold;font-size:.875rem}
.btn:hover{background:#22c55e}
table{width:100%;border-collapse:collapse;font-size:.875rem}
th,td{text-align:left;padding:.5rem .75rem;border-bottom:1px solid #1e293b}
th{color:#64748b;font-weight:normal}
tr:hover td{background:#1e293b}
.pending{color:#facc15}.running{color:#60a5fa;animation:pulse 1s infinite}
.success{color:#4ade80}.failed{color:#f87171}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.5}}
a{color:#60a5fa;text-decoration:none}
a:hover{text-decoration:underline}
.tag{display:inline-block;background:#1e293b;border:1px solid #334155;padding:0 .4rem;border-radius:3px;font-size:.75rem}
</style>
</head>
<body>
<header><h1>miraeboy build server</h1></header>
<main>

<h2>빌드 트리거</h2>
<div class="form-row">
  <input id="repoUrl" placeholder="Repo URL" style="width:320px" value="">
  <input id="ref" placeholder="ref" value="main" style="width:100px">
  <input id="version" placeholder="version" value="v1.0.0" style="width:100px">
</div>
<div class="form-row">
  <input id="platforms" value="linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64" style="width:500px">
  <small style="color:#64748b">플랫폼 (쉼표 구분)</small>
</div>
<div class="form-row">
  <input id="apiKey" placeholder="API Key" type="password" style="width:200px">
  <button class="btn" onclick="triggerBuild()">빌드 시작</button>
</div>

<h2>빌드 목록 <span style="color:#334155;font-size:.75rem" id="lastUpdate"></span></h2>
<table>
<thead><tr><th>ID</th><th>버전</th><th>Ref</th><th>상태</th><th>생성</th><th>아티팩트</th></tr></thead>
<tbody id="buildList"></tbody>
</table>

</main>
<script>
let savedKey = localStorage.getItem('buildApiKey') || '';
document.getElementById('apiKey').value = savedKey;
document.getElementById('apiKey').addEventListener('change', e => {
  localStorage.setItem('buildApiKey', e.target.value);
});

async function triggerBuild() {
  const apiKey = document.getElementById('apiKey').value;
  if (!apiKey) { alert('API Key를 입력하세요'); return; }
  const body = {
    repo_url:  document.getElementById('repoUrl').value,
    ref:       document.getElementById('ref').value,
    version:   document.getElementById('version').value,
    platforms: document.getElementById('platforms').value.split(',').map(s=>s.trim()).filter(Boolean),
  };
  if (!body.repo_url) { alert('Repo URL을 입력하세요'); return; }
  const r = await fetch('/api/build', {
    method: 'POST',
    headers: {'Content-Type':'application/json','X-API-Key':apiKey},
    body: JSON.stringify(body),
  });
  if (r.ok) { await load(); }
  else { alert('Error: ' + await r.text()); }
}

async function load() {
  const r = await fetch('/api/builds');
  if (!r.ok) return;
  const builds = await r.json();
  const tb = document.getElementById('buildList');
  tb.innerHTML = (builds || []).map(b => {
    const arts = (b.jobs||[]).filter(j=>j.artifact).map(j=>
      '<a href="/api/artifacts/'+b.id+'/'+j.artifact+'" title="'+j.platform+'">'+
      j.platform.replace('/','-')+'</a>'
    ).join(' ');
    return '<tr>'+
      '<td><span class="tag">'+b.id+'</span></td>'+
      '<td>'+b.version+'</td>'+
      '<td>'+b.ref+'</td>'+
      '<td class="'+b.status+'">'+b.status+'</td>'+
      '<td>'+new Date(b.created_at).toLocaleString('ko-KR')+'</td>'+
      '<td>'+arts+'</td>'+
      '</tr>';
  }).join('');
  document.getElementById('lastUpdate').textContent = '('+new Date().toLocaleTimeString('ko-KR')+' 기준)';
}

load();
setInterval(load, 5000);
</script>
</body>
</html>`

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	addr      := flag.String("addr", ":8500", "listen address")
	apiKey    := flag.String("api-key", "", "API key for authentication (required)")
	artifacts := flag.String("artifacts", "./artifacts", "artifact storage directory")
	flag.Parse()

	if *apiKey == "" {
		log.Fatal("--api-key is required")
	}

	st := newStore(*artifacts, *apiKey)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/build",                    st.handleTrigger)
	mux.HandleFunc("GET /api/builds",                    st.handleListBuilds)
	mux.HandleFunc("GET /api/builds/{id}",               st.handleGetBuild)
	mux.HandleFunc("GET /api/artifacts/{build}/{file}",  st.handleArtifact)
	mux.HandleFunc("POST /api/agent/poll",               st.handlePoll)
	mux.HandleFunc("POST /api/agent/jobs/{id}/done",     st.handleDone)
	mux.HandleFunc("GET /",                              st.handleDashboard)

	log.Printf("Build server listening on %s (artifacts: %s)", *addr, *artifacts)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
