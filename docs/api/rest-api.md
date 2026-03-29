# REST API 레퍼런스

miraeboy 서버의 REST API 엔드포인트를 설명합니다.

---

## 기본 정보

| 항목 | 값 |
|------|-----|
| 기본 URL | `http://server:9300` |
| 인증 방식 | Bearer JWT 토큰 |
| 콘텐츠 타입 | `application/json` |
| 토큰 TTL | 24시간 |

---

## 인증

### 토큰 획득

```http
POST /api/auth/login
Content-Type: application/json
```

요청:

```json
{
  "username": "admin",
  "password": "admin123"
}
```

응답 (200 OK):

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2024-03-16T10:00:00Z"
}
```

**예시:**

```bash
TOKEN=$(curl -s -X POST http://localhost:9300/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.token')

echo "Token: $TOKEN"
```

### OIDC 로그인 (브라우저)

```http
GET /api/auth/oidc/login
```

브라우저를 OIDC 프로바이더 로그인 페이지로 리디렉트합니다.

### OIDC 콜백

```http
GET /api/auth/oidc/callback?code=...&state=...
```

OIDC 인증 완료 후 IdP가 이 URL로 리디렉트합니다. 직접 호출하지 않습니다.

---

## 헬스 체크

```http
GET /ping
```

응답 (200 OK):

```json
{"status": "ok"}
```

```bash
curl http://localhost:9300/ping
```

---

## 사용자 API

모든 사용자 API는 관리자 권한이 필요합니다.

### 사용자 목록 조회

```http
GET /api/users
Authorization: Bearer <token>
```

응답:

```json
[
  {
    "username": "admin",
    "admin": true,
    "created_at": "2024-01-01T00:00:00Z"
  },
  {
    "username": "dev1",
    "admin": false,
    "created_at": "2024-01-10T09:00:00Z"
  }
]
```

### 사용자 생성

```http
POST /api/users
Authorization: Bearer <token>
Content-Type: application/json
```

요청:

```json
{
  "username": "newuser",
  "password": "securePassword123",
  "admin": false
}
```

응답 (201 Created):

```json
{
  "username": "newuser",
  "admin": false,
  "created_at": "2024-03-15T10:00:00Z"
}
```

오류 응답 (409 Conflict — 이미 존재):

```json
{
  "error": "user already exists"
}
```

### 사용자 조회

```http
GET /api/users/{username}
Authorization: Bearer <token>
```

응답:

```json
{
  "username": "dev1",
  "admin": false,
  "created_at": "2024-01-10T09:00:00Z"
}
```

### 사용자 수정

```http
PATCH /api/users/{username}
Authorization: Bearer <token>
Content-Type: application/json
```

요청 (변경할 필드만 포함):

```json
{
  "password": "newPassword456",
  "admin": true
}
```

응답 (200 OK):

```json
{
  "username": "dev1",
  "admin": true,
  "updated_at": "2024-03-15T11:00:00Z"
}
```

### 사용자 삭제

```http
DELETE /api/users/{username}
Authorization: Bearer <token>
```

응답 (204 No Content)

---

## 리포지토리 API

### 리포지토리 목록 조회

```http
GET /api/repos
Authorization: Bearer <token>
```

응답:

```json
[
  {
    "name": "extralib",
    "description": "사내 외부 라이브러리",
    "owner": "dev1",
    "anonymous_access": "none",
    "allowed_namespaces": ["sc"],
    "allowed_channels": ["dev", "release"],
    "created_at": "2024-01-01T00:00:00Z"
  }
]
```

### 리포지토리 생성

```http
POST /api/repos
Authorization: Bearer <token>
Content-Type: application/json
```

요청:

```json
{
  "name": "extralib",
  "description": "사내 외부 라이브러리",
  "owner": "dev1",
  "allowed_namespaces": ["sc"],
  "allowed_channels": ["dev", "release"],
  "anonymous_access": "none",
  "git": {
    "url": "https://github.com/corp/conan-recipes.git",
    "branch": "main",
    "token": "ghp_xxxx"
  }
}
```

응답 (201 Created):

```json
{
  "name": "extralib",
  "description": "사내 외부 라이브러리",
  "owner": "dev1",
  "anonymous_access": "none",
  "created_at": "2024-03-15T10:00:00Z"
}
```

### 리포지토리 조회

```http
GET /api/repos/{repo}
Authorization: Bearer <token>
```

### 리포지토리 수정

```http
PATCH /api/repos/{repo}
Authorization: Bearer <token>
Content-Type: application/json
```

요청 (변경할 필드만):

```json
{
  "description": "새 설명",
  "anonymous_access": "read",
  "allowed_channels": ["dev", "release", "stable"]
}
```

### 리포지토리 삭제

```http
DELETE /api/repos/{repo}?force=true
Authorization: Bearer <token>
```

쿼리 파라미터:
- `force=true`: 패키지가 있어도 강제 삭제

---

## 리포지토리 멤버 API

### 멤버 목록 조회

```http
GET /api/repos/{repo}/members
Authorization: Bearer <token>
```

응답:

```json
[
  {
    "username": "dev1",
    "permission": "owner",
    "added_at": "2024-01-01T00:00:00Z"
  },
  {
    "username": "ci",
    "permission": "write",
    "added_at": "2024-01-10T09:00:00Z"
  }
]
```

### 멤버 추가

```http
POST /api/repos/{repo}/members
Authorization: Bearer <token>
Content-Type: application/json
```

요청:

```json
{
  "username": "dev2",
  "permission": "write"
}
```

응답 (201 Created):

```json
{
  "username": "dev2",
  "permission": "write",
  "added_at": "2024-03-15T10:00:00Z"
}
```

### 멤버 권한 변경

```http
PUT /api/repos/{repo}/members/{username}
Authorization: Bearer <token>
Content-Type: application/json
```

요청:

```json
{
  "permission": "delete"
}
```

### 멤버 삭제

```http
DELETE /api/repos/{repo}/members/{username}
Authorization: Bearer <token>
```

---

## 빌드 API

### 빌드 목록 조회

```http
GET /api/builds
Authorization: Bearer <token>
```

응답:

```json
[
  {
    "id": 42,
    "repository": "extralib",
    "status": "success",
    "platform": "linux/amd64",
    "git_url": "https://github.com/mycompany/mylib.git",
    "git_ref": "v1.2.0",
    "started_at": "2024-03-15T10:00:00Z",
    "finished_at": "2024-03-15T10:05:30Z"
  }
]
```

### 빌드 트리거

```http
POST /api/builds
Authorization: Bearer <token>
Content-Type: application/json
```

요청:

```json
{
  "repository": "extralib",
  "git_url": "https://github.com/mycompany/mylib.git",
  "git_ref": "main",
  "platforms": ["linux/amd64", "windows/amd64"]
}
```

응답 (202 Accepted):

```json
{
  "id": 43,
  "status": "pending",
  "repository": "extralib",
  "platform": "linux/amd64",
  "created_at": "2024-03-15T11:00:00Z"
}
```

### 빌드 상세 조회

```http
GET /api/builds/{id}
Authorization: Bearer <token>
```

응답:

```json
{
  "id": 43,
  "repository": "extralib",
  "status": "running",
  "platform": "linux/amd64",
  "git_url": "https://github.com/mycompany/mylib.git",
  "git_ref": "main",
  "log": "[INFO] Cloning repository...\n[INFO] Running conan create...",
  "started_at": "2024-03-15T11:00:05Z",
  "finished_at": null
}
```

---

## Conan v2 API

Conan 클라이언트가 사용하는 API입니다. 일반적으로 `conan` 명령으로 간접 호출합니다.

**기본 경로:** `/api/conan/{repository}/v2/`

### 인증 토큰 발급

```http
GET /api/conan/{repository}/v2/users/authenticate
Authorization: Basic <base64(username:password)>
```

응답:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 자격 증명 확인

```http
GET /api/conan/{repository}/v2/users/check_credentials
Authorization: Bearer <token>
```

응답 (200 OK): 인증 성공

### 패키지 검색

```http
GET /api/conan/{repository}/v2/conans/search?q={query}
Authorization: Bearer <token>
```

쿼리 파라미터:
- `q`: 검색어 (와일드카드 `*` 지원)

```bash
curl "http://localhost:9300/api/conan/extralib/v2/conans/search?q=mylib*" \
  -H "Authorization: Bearer $TOKEN"
```

### 레시피 리비전 목록

```http
GET /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions
Authorization: Bearer <token>
```

응답:

```json
{
  "revisions": [
    {
      "revision": "abc123def456",
      "time": "2024-03-15T10:00:00.000000+0000"
    }
  ]
}
```

### 최신 레시피 리비전 조회

```http
GET /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/latest
Authorization: Bearer <token>
```

### 레시피 파일 목록

```http
GET /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/files
Authorization: Bearer <token>
```

### 레시피 파일 다운로드

```http
GET /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/files/{filename}
Authorization: Bearer <token>
```

### 레시피 파일 업로드

```http
PUT /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/files/{filename}
Authorization: Bearer <token>
Content-Type: application/octet-stream
```

### 레시피 삭제

```http
DELETE /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}
Authorization: Bearer <token>
```

### 패키지 리비전 목록

```http
GET /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/packages/{pkgid}/revisions
Authorization: Bearer <token>
```

### 패키지 파일 업로드

```http
PUT /api/conan/{repo}/v2/conans/{name}/{version}/{namespace}/{channel}/revisions/{rrev}/packages/{pkgid}/revisions/{prev}/files/{filename}
Authorization: Bearer <token>
Content-Type: application/octet-stream
```

---

## Cargo API

**기본 경로:** `/cargo/{repository}/`

### 레지스트리 설정 조회

```http
GET /cargo/{repository}/config.json
```

응답:

```json
{
  "dl": "http://server:9300/cargo/{repository}/dl",
  "api": "http://server:9300/cargo/{repository}"
}
```

### 크레이트 인덱스 조회

```http
GET /cargo/{repository}/{prefix}/{name}
Authorization: Bearer <token>
```

응답 (Newline-Delimited JSON):

```
{"name":"myutils","vers":"0.1.0","deps":[],"cksum":"sha256:abc...","features":{},"yanked":false}
{"name":"myutils","vers":"0.2.0","deps":[],"cksum":"sha256:def...","features":{},"yanked":false}
```

### 크레이트 검색

```http
GET /cargo/{repository}/api/v1/crates?q={query}&per_page={n}
Authorization: Bearer <token>
```

### 크레이트 배포

```http
PUT /cargo/{repository}/api/v1/crates/new
Authorization: Bearer <token>
Content-Type: application/octet-stream
```

### 크레이트 다운로드

```http
GET /cargo/{repository}/dl/{name}/{version}/download
Authorization: Bearer <token>
```

### 크레이트 Yank

```http
DELETE /cargo/{repository}/api/v1/crates/{name}/{version}/yank
Authorization: Bearer <token>
```

응답:

```json
{"ok": true}
```

### 크레이트 Unyank

```http
PUT /cargo/{repository}/api/v1/crates/{name}/{version}/unyank
Authorization: Bearer <token>
```

---

## 오류 응답 형식

모든 API 오류는 동일한 형식을 사용합니다.

```json
{
  "error": "오류 메시지"
}
```

| HTTP 상태 코드 | 의미 |
|----------------|------|
| `400 Bad Request` | 잘못된 요청 (파라미터 오류 등) |
| `401 Unauthorized` | 인증 필요 또는 토큰 만료 |
| `403 Forbidden` | 권한 부족 |
| `404 Not Found` | 리소스를 찾을 수 없음 |
| `409 Conflict` | 이미 존재하는 리소스 |
| `500 Internal Server Error` | 서버 내부 오류 |
