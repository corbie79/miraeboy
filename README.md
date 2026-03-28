# miraeboy

**한국어** | [English](README.en.md)

Conan v2 호환 사내 패키지 서버. Go로 작성된 단일 바이너리이며 웹 관리 UI가 내장되어 있습니다.

## 특징

- **Conan v2 프로토콜** 완전 호환 — 기존 Conan 클라이언트를 그대로 사용
- **리포지토리 단위 관리** — 여러 리포지토리를 독립적으로 운영
- **네임스페이스 / 채널 제한** — 리포지토리별 `@namespace/channel` 허용 목록 설정
- **JWT 인증** — 역할 기반 권한 (read / write / delete / owner)
- **글로벌 admin** — 모든 리포지토리에 자동 최고 권한
- **웹 관리 UI** — 리포지토리 생성/설정, 멤버 초대, 패키지 검색
- **단일 바이너리 배포** — 웹 UI가 바이너리에 내장 (`embed.FS`)

## JFrog Artifactory와의 설계 차이

### URL 구조

JFrog는 Artifactory 자체의 네임스페이스(`/artifactory/`)가 앞에 붙습니다.

```
JFrog:    http://server:8081/artifactory/api/conan/extralib
miraeboy: http://server:9300/api/conan/extralib
```

Conan 클라이언트는 등록된 remote URL 뒤에 `/v2/...`를 자동으로 붙여 호출합니다.
miraeboy는 이 규칙을 그대로 따르되 불필요한 경로 계층을 제거했습니다.

---

### `@namespace/channel` 처리 방식

JFrog는 `@username/channel`에서 `username`을 **Artifactory 계정**과 연결합니다.
패키지가 특정 사용자 소유로 귀속되며, 다른 사용자가 같은 namespace로 업로드하려면
별도의 권한 설정이 필요합니다.

miraeboy는 `@namespace/channel`을 **조직 레이블(폴더)** 로만 취급합니다.

```
zlib/1.3.1@sc/dev
              ↑
         시스템 계정이 아닌 팀/조직 식별자
```

- `sc`는 사내 팀명, 프로젝트명 등 자유롭게 사용하는 네임스페이스
- 업로드 권한은 리포지토리 멤버 권한으로만 결정됨
- `allowed_namespaces: ["sc"]`로 허용 목록을 강제하여 실수 방지

이 방식은 사내 환경에서 패키지를 개인 계정이 아닌 **팀 자산**으로 관리하기에 적합합니다.

---

### 권한 모델

| 항목 | JFrog Artifactory | miraeboy |
|------|-------------------|----------|
| 권한 단위 | Permission Target (복잡한 규칙 조합) | 리포지토리별 멤버 역할 |
| 역할 | Deploy/Delete/Manage/Admin 등 | read / write / delete / owner / admin |
| 사용자 그룹 | 별도 그룹 엔티티 생성 후 권한 연결 | 리포지토리 멤버 직접 초대 |
| 익명 접근 | 리포지토리 설정 + 권한 타겟 조합 | `anonymous_access: read` 한 줄 |

JFrog의 권한 모델은 수백 개의 리포지토리와 복잡한 조직 구조를 다루기 위해 설계되어
소규모 팀에서는 오히려 설정 부담이 큽니다. miraeboy는 "리포지토리 오너가 멤버를 초대하고
권한을 부여한다"는 단순한 모델만 제공합니다.

---

### 설정 관리 방식

JFrog는 UI 또는 REST API로 모든 상태를 관리하며, 설정을 코드로 버전 관리하려면
별도 IaC 도구(Terraform Artifactory Provider 등)가 필요합니다.

miraeboy는 `config.yaml`이 **시드(seed)** 역할을 합니다.

```
config.yaml  →  서버 시작 시 디스크에 없는 리포지토리만 생성
이후 변경     →  API / 웹 UI로 관리, _repos/{name}.json에 저장
```

초기 리포지토리 구성을 코드로 관리하면서도, 운영 중 변경은 UI로 처리할 수 있습니다.

---

### 배포 복잡도

| 항목 | JFrog Artifactory (OSS) | miraeboy |
|------|-------------------------|----------|
| 런타임 | JVM (Java 11+) | 없음 |
| 의존성 | 외부 DB 옵션, 설정 파일 다수 | 없음 |
| 배포 단위 | WAR / Docker 이미지 | 단일 바이너리 |
| 웹 UI | 별도 번들 | 바이너리에 내장 |
| 최소 메모리 | ~512MB | ~20MB |

miraeboy는 소규모 사내 팀이 **별도 인프라 없이 빠르게 운영**하는 데 초점을 맞춥니다.
JFrog가 제공하는 가상 리포지토리, 원격 프록시, HA 클러스터, 감사 로그 등 엔터프라이즈
기능은 의도적으로 포함하지 않았습니다.

---

## 빠른 시작

### 1. 빌드

```bash
# 프론트엔드 빌드 (Go 빌드 전 필요)
cd web && npm install && npm run build && cd ..

# Go 바이너리 빌드
go build -o miraeboy .
```

### 2. 실행

```bash
./miraeboy
# Conan2 server listening on :9300
```

웹 관리 UI: `http://localhost:9300`

### 3. Conan 클라이언트 등록

```bash
conan remote add extralib http://localhost:9300/api/conan/extralib
conan remote login extralib -u admin
```

## 설정 (config.yaml)

```yaml
server:
  address: ":9300"
  storage_path: "./data"

auth:
  jwt_secret: "your-strong-secret-here"   # 반드시 변경
  users:
    - username: "admin"
      password: "strongpassword"
      admin: true

    - username: "dev1"
      password: "devpassword"
      admin: false

repositories:
  - name: "extralib"
    description: "사내 외부 라이브러리"
    owner: "dev1"
    allowed_namespaces: ["sc"]           # 빈 배열 = 제한 없음
    allowed_channels: ["dev", "release"] # 빈 배열 = 제한 없음
    anonymous_access: "none"             # "none" | "read"
    members:
      - username: "dev1"
        permission: "owner"
      - username: "ci"
        permission: "write"
```

> config.yaml의 `repositories`는 서버 최초 시작 시 디스크에 없는 리포지토리만 생성합니다.
> 이후 상태는 API / 웹 UI로 관리합니다.

## URL 구조

| 용도 | URL |
|------|-----|
| 웹 관리 UI | `http://server:9300/` |
| Conan 리모트 등록 | `http://server:9300/api/conan/{repository}` |
| 리포지토리 관리 API | `http://server:9300/api/repos` |
| 로그인 API | `http://server:9300/api/auth/login` |

## 패키지 레퍼런스 형식

```
{name}/{version}@{namespace}/{channel}

예: zlib/1.3.1@sc/dev
         ↑         ↑    ↑
       버전   namespace  channel
```

- **namespace** (`sc`) — 리포지토리의 `allowed_namespaces`로 제한
- **channel** (`dev`, `release`) — 리포지토리의 `allowed_channels`로 제한

## 권한 체계

| 권한 | 다운로드 | 업로드 | 삭제 | 멤버 관리 |
|------|----------|--------|------|-----------|
| `read` | ✅ | ❌ | ❌ | ❌ |
| `write` | ✅ | ✅ | ❌ | ❌ |
| `delete` | ✅ | ✅ | ✅ | ❌ |
| `owner` | ✅ | ✅ | ✅ | ✅ |
| `admin` | ✅ | ✅ | ✅ | ✅ (전체) |

## REST API

### 인증

```bash
# 토큰 발급
curl -X POST http://server:9300/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 이후 요청에 헤더 추가
-H "Authorization: Bearer <token>"
```

### 리포지토리

| Method | Path | 설명 | 필요 권한 |
|--------|------|------|-----------|
| `GET` | `/api/repos` | 목록 조회 | admin |
| `POST` | `/api/repos` | 생성 | admin |
| `GET` | `/api/repos/{name}` | 상세 조회 | admin |
| `PATCH` | `/api/repos/{name}` | 설정 수정 | owner / admin |
| `DELETE` | `/api/repos/{name}?force=true` | 삭제 | admin |

### 멤버 관리

| Method | Path | 설명 |
|--------|------|------|
| `GET` | `/api/repos/{name}/members` | 멤버 목록 |
| `POST` | `/api/repos/{name}/members` | 초대 |
| `PUT` | `/api/repos/{name}/members/{username}` | 권한 변경 |
| `DELETE` | `/api/repos/{name}/members/{username}` | 제거 |

### 예시

```bash
TOKEN="..."

# 리포지토리 생성
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

# 멤버 초대
curl -X POST http://server:9300/api/repos/extralib/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username": "ci", "permission": "write"}'

# allowed_channels 수정
curl -X PATCH http://server:9300/api/repos/extralib \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"allowed_channels": ["dev", "release", "stable"]}'
```

## 데이터 저장 구조

```
data/
  _repos/
    extralib.json          ← 리포지토리 설정 + 멤버 목록
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

## OIDC SSO 연동

Keycloak, Azure AD 등 OpenID Connect 호환 IdP와 연동할 수 있습니다.
로컬 `users:` 로그인은 OIDC 활성화 여부와 무관하게 항상 사용 가능합니다.

### 흐름

```
브라우저 → GET /api/auth/oidc/login
         → (IdP로 redirect)
         → 사용자 로그인 완료
         → GET /api/auth/oidc/callback?code=...
         → miraeboy 내부 JWT 발급
         → 웹 UI로 redirect (#/auth/callback?token=...)
```

Conan CLI는 로컬 계정(`POST /api/auth/login`)으로 계속 인증합니다.
CI 서비스 계정은 `config.yaml`의 `users:`에 등록하면 됩니다.

### config.yaml 설정

```yaml
auth:
  jwt_secret: "your-strong-secret-here"
  oidc:
    issuer: "https://keycloak.example.com/realms/company"
    client_id: "miraeboy"
    client_secret: "your-client-secret"
    redirect_url: "http://miraeboy.example.com/api/auth/oidc/callback"

    # 그룹 클레임 이름 (Keycloak 기본값: "groups", Azure AD: "groups")
    groups_claim: "groups"

    # 이 그룹에 속하면 전체 admin
    admin_groups: ["miraeboy-admin"]

    # OIDC 그룹 → 리포지토리 권한 매핑
    group_mappings:
      - group: "devteam"
        repository: "extralib"
        permission: "write"
      - group: "readonly-all"
        repository: "*"          # 모든 리포지토리에 read
        permission: "read"
      - group: "ci-team"
        repository: "extralib"
        permission: "write"
```

### OIDC 그룹과 permission 매핑

| OIDC 그룹 | repository | permission | 결과 |
|-----------|------------|------------|------|
| `miraeboy-admin` | — | — | 전체 admin |
| `devteam` | `extralib` | `write` | extralib에 업로드 가능 |
| `readonly-all` | `*` | `read` | 모든 리포지토리 읽기 |

한 사용자가 여러 그룹에 속하면 **가장 높은 권한**이 적용됩니다.

### Keycloak 설정 포인트

1. Client 생성: `miraeboy`, Valid Redirect URI: `http://miraeboy.example.com/api/auth/oidc/callback`
2. Client Scope에 `groups` mapper 추가 → ID 토큰에 그룹 포함
3. Windows AD 연동: User Federation → LDAP → AD 서버 연결
4. Azure AD 연동: Identity Providers → OpenID Connect → Azure tenant endpoint

### Azure AD 직결 (Keycloak 없이)

```yaml
oidc:
  issuer: "https://login.microsoftonline.com/{tenant-id}/v2.0"
  client_id: "{app-registration-client-id}"
  client_secret: "{client-secret}"
  redirect_url: "http://miraeboy.example.com/api/auth/oidc/callback"
  groups_claim: "groups"    # App Registration → Token configuration → Groups claim 추가 필요
  admin_groups: ["{admin-group-object-id}"]
```

> Azure AD는 그룹을 GUID로 포함합니다. `admin_groups`에 그룹 Object ID를 사용하세요.

---

## Active-Passive HA 구성 (S3 백엔드)

두 노드가 동일한 S3 버킷을 공유합니다. 로드밸런서가 쓰기 요청은 Primary로, 읽기 요청은 양쪽으로 분산합니다.

### 구성 개요

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
              │  S3 버킷          │
              │  (MinIO / AWS)   │
              └─────────────────┘
```

- **Primary** (`node_role: primary`): 읽기 + 쓰기 모두 처리
- **Replica** (`node_role: replica`): 읽기만 처리. 쓰기 요청 수신 시 `503 Service Unavailable` 반환

### config.yaml — Primary 노드

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

### config.yaml — Replica 노드

Primary와 동일하되 `node_role`만 변경합니다:

```yaml
server:
  address: ":9300"
  node_role: "replica"   # ← 이것만 다름
  s3:
    endpoint: "minio.example.com:9000"
    bucket: "miraeboy"
    access_key_id: "access-key"
    secret_access_key: "secret-key"
    use_ssl: false
    region: ""

auth:
  jwt_secret: "your-strong-secret-here"   # Primary와 동일해야 JWT 검증 가능
  users: ...
```

### nginx 라우팅 예시

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

    # 쓰기 요청 → Primary만
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

> **주의**: JWT secret은 모든 노드에서 동일해야 합니다. Primary에서 발급한 토큰을 Replica에서도 검증할 수 있습니다.

---

## 개발

```bash
# 백엔드 개발 서버 (hot reload 없음)
go run .

# 프론트엔드 개발 서버 (API 프록시 → localhost:9300)
cd web && npm run dev
```

## 기술 스택

| 구분 | 기술 |
|------|------|
| Backend | Go 1.22, `net/http` (표준 라이브러리) |
| Auth | JWT (`golang-jwt/jwt/v5`) |
| Config | YAML (`gopkg.in/yaml.v3`) |
| Storage | 로컬 파일시스템 또는 S3 호환 (`minio-go/v7`) |
| Frontend | Svelte 5, Vite, Tailwind CSS |
| 배포 | 단일 바이너리 (`embed.FS`) |
