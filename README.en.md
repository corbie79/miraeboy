# miraeboy

[한국어](README.md) | **English**

An in-house Conan v2 compatible package server. A single binary written in Go with a built-in web management UI.

## Features

- **Conan v2 protocol** fully compatible — use your existing Conan client as-is
- **Repository-based management** — operate multiple repositories independently
- **Namespace / channel whitelisting** — per-repository `@namespace/channel` allowlists
- **JWT authentication** — role-based permissions (read / write / delete / owner)
- **Global admin** — automatic full access across all repositories
- **Web management UI** — create/configure repositories, invite members, search packages
- **Single binary deployment** — web UI embedded in the binary (`embed.FS`)

## Design Differences from JFrog Artifactory

### URL Structure

JFrog prepends Artifactory's own namespace (`/artifactory/`) to all paths.

```
JFrog:    http://server:8081/artifactory/api/conan/extralib
miraeboy: http://server:9300/api/conan/extralib
```

The Conan client automatically appends `/v2/...` to the registered remote URL.
miraeboy follows this convention while removing unnecessary path layers.

---

### `@namespace/channel` Handling

JFrog ties the `username` in `@username/channel` to an **Artifactory account**.
Packages are owned by a specific user, and uploading under the same namespace
from a different account requires separate permission configuration.

miraeboy treats `@namespace/channel` purely as an **organizational label (folder)**.

```
zlib/1.3.1@sc/dev
              ↑
     team/org identifier, not a system account
```

- `sc` is a namespace freely chosen as a team name, project name, etc.
- Upload permissions are determined solely by repository member roles
- `allowed_namespaces: ["sc"]` enforces an allowlist to prevent mistakes

This approach is well suited to managing packages as **team assets** rather than
personal account assets in an internal environment.

---

### Permission Model

| Item | JFrog Artifactory | miraeboy |
|------|-------------------|----------|
| Permission unit | Permission Targets (complex rule combinations) | Per-repository member role |
| Roles | Deploy / Delete / Manage / Admin, etc. | read / write / delete / owner / admin |
| User groups | Create separate group entity, then attach permissions | Direct member invitation per repository |
| Anonymous access | Repository settings + Permission Target combination | Single line: `anonymous_access: read` |

JFrog's permission model is designed for hundreds of repositories with complex
org structures — the setup overhead is significant for small teams. miraeboy
provides only a simple model: "repository owner invites members and grants permissions."

---

### Configuration Management

JFrog manages all state via UI or REST API. Version-controlling configuration as
code requires separate IaC tooling (e.g., Terraform Artifactory Provider).

In miraeboy, `config.yaml` acts as a **seed**.

```
config.yaml  →  creates repositories at startup only if they don't exist on disk
later changes →  managed via API / web UI, persisted in _repos/{name}.json
```

You can manage the initial repository setup as code while handling ongoing
changes through the UI.

---

### Deployment Complexity

| Item | JFrog Artifactory (OSS) | miraeboy |
|------|-------------------------|----------|
| Runtime | JVM (Java 11+) | none |
| Dependencies | Optional external DB, multiple config files | none |
| Deployment unit | WAR / Docker image | single binary |
| Web UI | Separate bundle | embedded in binary |
| Minimum memory | ~512MB | ~20MB |

miraeboy focuses on letting small internal teams **operate quickly without extra infrastructure**.
Enterprise features provided by JFrog — virtual repositories, remote proxies, HA clusters,
audit logs — are intentionally excluded.

---

## Quick Start

### 1. Build

```bash
# Build the frontend first (required before Go build)
cd web && npm install && npm run build && cd ..

# Build the Go binary
go build -o miraeboy .
```

### 2. Run

```bash
./miraeboy
# Conan2 server listening on :9300
```

Web management UI: `http://localhost:9300`

### 3. Register a Conan Remote

```bash
conan remote add extralib http://localhost:9300/api/conan/extralib
conan remote login extralib -u admin
```

## Configuration (config.yaml)

```yaml
server:
  address: ":9300"
  storage_path: "./data"

auth:
  jwt_secret: "your-strong-secret-here"   # change this
  users:
    - username: "admin"
      password: "strongpassword"
      admin: true

    - username: "dev1"
      password: "devpassword"
      admin: false

repositories:
  - name: "extralib"
    description: "Internal third-party libraries"
    owner: "dev1"
    allowed_namespaces: ["sc"]           # empty array = no restriction
    allowed_channels: ["dev", "release"] # empty array = no restriction
    anonymous_access: "none"             # "none" | "read"
    members:
      - username: "dev1"
        permission: "owner"
      - username: "ci"
        permission: "write"
```

> `repositories` in config.yaml only creates repositories at first startup if they
> don't already exist on disk. All subsequent state is managed via API / web UI.

## URL Structure

| Purpose | URL |
|---------|-----|
| Web management UI | `http://server:9300/` |
| Conan remote registration | `http://server:9300/api/conan/{repository}` |
| Repository management API | `http://server:9300/api/repos` |
| Login API | `http://server:9300/api/auth/login` |

## Package Reference Format

```
{name}/{version}@{namespace}/{channel}

Example: zlib/1.3.1@sc/dev
                    ↑    ↑
               namespace  channel
```

- **namespace** (`sc`) — restricted by the repository's `allowed_namespaces`
- **channel** (`dev`, `release`) — restricted by the repository's `allowed_channels`

## Permission System

| Permission | Download | Upload | Delete | Manage Members |
|------------|----------|--------|--------|----------------|
| `read` | ✅ | ❌ | ❌ | ❌ |
| `write` | ✅ | ✅ | ❌ | ❌ |
| `delete` | ✅ | ✅ | ✅ | ❌ |
| `owner` | ✅ | ✅ | ✅ | ✅ |
| `admin` | ✅ | ✅ | ✅ | ✅ (global) |

## REST API

### Authentication

```bash
# Obtain a token
curl -X POST http://server:9300/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# Include in subsequent requests
-H "Authorization: Bearer <token>"
```

### Repositories

| Method | Path | Description | Required Permission |
|--------|------|-------------|---------------------|
| `GET` | `/api/repos` | List repositories | admin |
| `POST` | `/api/repos` | Create repository | admin |
| `GET` | `/api/repos/{name}` | Get repository details | admin |
| `PATCH` | `/api/repos/{name}` | Update settings | owner / admin |
| `DELETE` | `/api/repos/{name}?force=true` | Delete repository | admin |

### Member Management

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/repos/{name}/members` | List members |
| `POST` | `/api/repos/{name}/members` | Invite member |
| `PUT` | `/api/repos/{name}/members/{username}` | Update permission |
| `DELETE` | `/api/repos/{name}/members/{username}` | Remove member |

### Examples

```bash
TOKEN="..."

# Create a repository
curl -X POST http://server:9300/api/repos \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "extralib",
    "owner": "dev1",
    "allowed_namespaces": ["sc"],
    "allowed_channels": ["dev", "release"],
    "anonymous_access": "none"
  }'

# Invite a member
curl -X POST http://server:9300/api/repos/extralib/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username": "ci", "permission": "write"}'

# Update allowed_channels
curl -X PATCH http://server:9300/api/repos/extralib \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"allowed_channels": ["dev", "release", "stable"]}'
```

## Data Storage Structure

```
data/
  _repos/
    extralib.json          ← repository config + member list
  extralib/
    zlib/1.3.1/sc/dev/
      recipe_revisions.json
      {rrev}/
        conanfile.py
        conanmanifest.txt
        ...
      packages/{pkgid}/{rrev}/
        pkg_revisions.json
        {prev}/
          conan_package.tgz
          ...
```

## Active-Passive HA Setup (S3 Backend)

Two nodes share the same S3 bucket. The load balancer routes write requests to
the Primary only and distributes read requests across both nodes.

> **This is entirely optional.** Without an S3 endpoint configured, miraeboy
> uses the local filesystem and runs as a single node with no changes needed.

### Architecture Overview

```
                 ┌─────────────────────────────────┐
Conan Client ───►│  Load Balancer (nginx / HAProxy) │
                 └──────────────┬──────────────────┘
                                │
              ┌─────────────────┼─────────────────┐
              │ GET/HEAD        │                  │ PUT/DELETE/POST/PATCH
              ▼                 ▼                  ▼
       ┌──────────────┐  ┌──────────────┐         │
       │ miraeboy      │  │ miraeboy      │◄────────┘
       │ node-1        │  │ node-2        │
       │ (replica)     │  │ (primary)     │
       └──────┬───────┘  └──────┬────────┘
              │                 │
              └────────┬────────┘
                       ▼
              ┌─────────────────┐
              │   S3 Bucket     │
              │ (MinIO / AWS)   │
              └─────────────────┘
```

- **Primary** (`node_role: primary`): handles both reads and writes
- **Replica** (`node_role: replica`): reads only. Returns `503 Service Unavailable` for write requests

### config.yaml — Primary Node

```yaml
server:
  address: ":9300"
  node_role: "primary"
  s3:
    endpoint: "minio.example.com:9000"
    bucket: "miraeboy"
    access_key_id: "access-key"
    secret_access_key: "secret-key"
    use_ssl: false
    region: ""

auth:
  jwt_secret: "your-strong-secret-here"
  users:
    - username: "admin"
      password: "strongpassword"
      admin: true
```

### config.yaml — Replica Node

Same as Primary, only `node_role` differs:

```yaml
server:
  address: ":9300"
  node_role: "replica"   # ← only this changes
  s3:
    endpoint: "minio.example.com:9000"
    bucket: "miraeboy"
    access_key_id: "access-key"
    secret_access_key: "secret-key"
    use_ssl: false
    region: ""

auth:
  jwt_secret: "your-strong-secret-here"   # must match Primary for JWT validation
  users: ...
```

### Building with S3 Support

S3 support requires the `-tags s3` build flag (pulls in `minio-go/v7`):

```bash
go get github.com/minio/minio-go/v7@v7.0.83
go build -tags s3 -o miraeboy .
```

The default build (`go build .`) compiles without S3 dependencies.
Configuring `s3.endpoint` on a binary built without `-tags s3` will return an error at startup.

### nginx Routing Example

```nginx
upstream primary {
    server miraeboy-primary:9300;
}

upstream all_nodes {
    server miraeboy-primary:9300;
    server miraeboy-replica:9300;
}

server {
    listen 9300;

    # Write requests → Primary only
    location ~ ^/(api/conan/.*/v2/conans/.*/revisions/.*/files/|api/repos) {
        limit_except GET HEAD {
            proxy_pass http://primary;
        }
        proxy_pass http://all_nodes;
    }

    location / {
        proxy_pass http://all_nodes;
    }
}
```

> **Note**: The JWT secret must be identical across all nodes. Tokens issued by
> the Primary can be validated by the Replica.

---

## Development

```bash
# Backend dev server (no hot reload)
go run .

# Frontend dev server (API proxy → localhost:9300)
cd web && npm run dev
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.22, `net/http` (standard library) |
| Auth | JWT (`golang-jwt/jwt/v5`) |
| Config | YAML (`gopkg.in/yaml.v3`) |
| Storage | Local filesystem or S3-compatible (`minio-go/v7`) |
| Frontend | Svelte 5, Vite, Tailwind CSS |
| Deployment | Single binary (`embed.FS`) |
