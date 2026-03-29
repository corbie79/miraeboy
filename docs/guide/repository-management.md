# 리포지토리 관리

miraeboy의 리포지토리 생성, 수정, 삭제 및 멤버 권한 관리 방법을 설명합니다.

---

## 리포지토리 개요

miraeboy에서 리포지토리는 패키지를 격리하여 관리하는 논리적 단위입니다.
각 리포지토리는 독립적인 URL을 가지며, 별도의 권한 설정이 가능합니다.

```
http://server:9300/api/conan/{repository}      ← Conan 레지스트리
sparse+http://server:9300/cargo/{repository}/  ← Cargo 레지스트리
```

---

## 리포지토리 목록 조회

```bash
mboy repo list
```

출력 예시:

```
NAME        OWNER   설명                    익명 접근
extralib    admin   사내 외부 라이브러리    none
rustlib     dev1    Rust 크레이트 저장소    read
shared      admin   공용 패키지             read
```

---

## 리포지토리 생성

### 기본 생성

```bash
mboy repo create --name extralib --owner admin
```

### 전체 옵션 사용

```bash
mboy repo create \
  --name extralib \
  --owner dev1 \
  --desc "사내 외부 C/C++ 라이브러리" \
  --anon none \
  --ns sc,mycompany \
  --ch dev,release,stable \
  --git-url https://github.com/corp/conan-recipes.git \
  --git-branch main \
  --git-token ghp_xxxx
```

### 옵션 설명

| 옵션 | 설명 | 기본값 |
|------|------|--------|
| `--name` | 리포지토리 이름 (필수) | — |
| `--owner` | 소유자 사용자명 (필수) | — |
| `--desc` | 설명 | — |
| `--anon` | 익명 접근 권한 (`none`, `read`, `write`) | `none` |
| `--ns` | 허용 네임스페이스 목록 (콤마 구분) | 모두 허용 |
| `--ch` | 허용 채널 목록 (콤마 구분) | 모두 허용 |
| `--git-url` | Git 레시피 동기화 URL | — |
| `--git-branch` | Git 브랜치 | `main` |
| `--git-token` | Git 인증 토큰 | — |

### allowed_namespaces와 allowed_channels

Conan 패키지에서 `namespace`(user)와 `channel`을 제한할 수 있습니다.

```
hello/1.0.0@sc/dev
              ^^ ^^^
              NS  CH (namespace/channel)
```

예: `sc` 네임스페이스, `dev` 또는 `release` 채널만 허용:

```bash
mboy repo create --name extralib --owner admin --ns sc --ch dev,release
```

이 설정 시 `hello/1.0.0@other/dev` 업로드는 거부됩니다.

!!! tip "빈 목록 = 전체 허용"
    `--ns`와 `--ch`를 지정하지 않으면 모든 네임스페이스/채널을 허용합니다.

---

## 리포지토리 상세 조회

```bash
mboy repo get extralib
```

출력 예시:

```
이름:         extralib
설명:         사내 외부 C/C++ 라이브러리
소유자:       dev1
익명 접근:    none
네임스페이스: [sc, mycompany]
채널:         [dev, release]
Git URL:      https://github.com/corp/conan-recipes.git
Git 브랜치:   main
멤버 수:      3
```

---

## 리포지토리 수정

```bash
# 설명 변경
mboy repo update extralib --desc "새 설명"

# 익명 접근 허용 (읽기)
mboy repo update extralib --anon read

# 허용 채널 추가
mboy repo update extralib --ch dev,release,stable

# 익명 접근 차단
mboy repo update extralib --anon none
```

---

## 리포지토리 삭제

```bash
# 빈 리포지토리 삭제
mboy repo delete extralib

# 패키지가 있는 리포지토리 강제 삭제
mboy repo delete extralib --force
```

!!! danger "강제 삭제 주의"
    `--force` 플래그는 리포지토리 내 **모든 패키지를 영구 삭제**합니다.
    삭제 전 반드시 백업을 확인하세요.

---

## 멤버 관리

리포지토리 소유자(owner) 또는 관리자(admin)만 멤버를 관리할 수 있습니다.

### 권한 계층

```
none  <  read  <  write  <  delete  <  owner
```

| 권한 | 패키지 다운로드 | 패키지 업로드 | 패키지 삭제 | 설정 변경 | 멤버 관리 |
|------|:---:|:---:|:---:|:---:|:---:|
| `none` | | | | | |
| `read` | ✓ | | | | |
| `write` | ✓ | ✓ | | | |
| `delete` | ✓ | ✓ | ✓ | | |
| `owner` | ✓ | ✓ | ✓ | ✓ | ✓ |

### 멤버 추가

```bash
# 읽기 권한으로 추가
mboy member add extralib dev2 read

# 쓰기 권한으로 추가
mboy member add extralib ci write

# 삭제 권한으로 추가
mboy member add extralib senior-dev delete

# 소유자 권한으로 추가
mboy member add extralib teamlead owner
```

### 멤버 목록 조회

```bash
mboy member list extralib
```

출력 예시:

```
USERNAME    권한      추가일
dev1        owner     2024-01-01 (소유자)
ci          write     2024-01-10
dev2        read      2024-02-01
```

### 권한 변경

```bash
# dev2의 권한을 write로 변경
mboy member update extralib dev2 write
```

### 멤버 제거

```bash
mboy member remove extralib dev2
```

---

## 익명 접근 설정

로그인하지 않은 사용자의 접근을 제어합니다.

| 설정값 | 효과 |
|--------|------|
| `none` | 로그인 필수 (기본값) |
| `read` | 비인증 사용자도 패키지 다운로드 가능 |
| `write` | 비인증 사용자도 패키지 업로드 가능 (보안 위험, 내부망 전용) |

```bash
# 내부망 전용 공용 리포지토리 (읽기 익명 허용)
mboy repo update shared --anon read
```

!!! warning "write 익명 접근"
    `anonymous_access: write`는 인터넷에 노출된 서버에서는 절대 사용하지 마세요.
    내부 네트워크 전용 환경에서만 사용하세요.

---

## REST API로 리포지토리 관리

### 리포지토리 목록 조회

```http
GET /api/repos
Authorization: Bearer <token>
```

```bash
curl http://localhost:9300/api/repos \
  -H "Authorization: Bearer $TOKEN"
```

### 리포지토리 생성 (관리자 전용)

```http
POST /api/repos
Authorization: Bearer <token>
Content-Type: application/json
```

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

### 리포지토리 수정

```http
PATCH /api/repos/{repo}
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "description": "수정된 설명",
  "anonymous_access": "read"
}
```

### 리포지토리 삭제

```http
DELETE /api/repos/{repo}?force=true
Authorization: Bearer <token>
```

### 멤버 추가

```http
POST /api/repos/{repo}/members
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "username": "dev2",
  "permission": "write"
}
```

### 멤버 권한 변경

```http
PUT /api/repos/{repo}/members/{username}
Authorization: Bearer <token>
Content-Type: application/json
```

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

## config.yaml로 초기 리포지토리 설정

서버 시작 시 config.yaml의 `repositories` 목록에 정의된 리포지토리가 자동으로 생성됩니다.

```yaml
repositories:
  - name: "extralib"
    description: "사내 외부 라이브러리"
    owner: "dev1"
    allowed_namespaces: ["sc"]
    allowed_channels: ["dev", "release"]
    anonymous_access: "none"
    members:
      - username: "ci"
        permission: "write"
      - username: "dev2"
        permission: "read"
    git:
      url: "https://github.com/corp/conan-recipes.git"
      branch: "main"
      token: "ghp_xxxx"

  - name: "rustlib"
    description: "사내 Rust 크레이트"
    owner: "admin"
    anonymous_access: "none"
    members: []
```

!!! note "설정 파일 우선순위"
    config.yaml에 정의된 리포지토리는 서버 시작 시 존재하지 않으면 생성됩니다.
    이미 존재하는 리포지토리는 덮어쓰지 않습니다.
