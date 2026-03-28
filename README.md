# miraeboy

Conan v2 호환 사내 패키지 서버. Go로 작성된 단일 바이너리이며 웹 관리 UI가 내장되어 있습니다.

## 특징

- **Conan v2 프로토콜** 완전 호환 — 기존 Conan 클라이언트를 그대로 사용
- **리포지토리 단위 관리** — 여러 리포지토리를 독립적으로 운영
- **네임스페이스 / 채널 제한** — 리포지토리별 `@namespace/channel` 허용 목록 설정
- **JWT 인증** — 역할 기반 권한 (read / write / delete / owner)
- **글로벌 admin** — 모든 리포지토리에 자동 최고 권한
- **웹 관리 UI** — 리포지토리 생성/설정, 멤버 초대, 패키지 검색
- **단일 바이너리 배포** — 웹 UI가 바이너리에 내장 (`embed.FS`)

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
| Frontend | Svelte 5, Vite, Tailwind CSS |
| 배포 | 단일 바이너리 (`embed.FS`) |
