# Cargo 레지스트리

miraeboy의 Cargo 스파스(sparse) 레지스트리 사용 방법을 설명합니다.

---

## 개요

miraeboy는 [RFC 3143](https://rust-lang.github.io/rfcs/3143-cargo-sparse-registry.html)의 Cargo 스파스 프로토콜을 구현합니다.
`sparse+` 접두사를 사용하는 HTTP 기반 레지스트리로, 기존 git 인덱스 방식보다 빠릅니다.

**기본 URL 패턴:**
```
sparse+http://server:9300/cargo/{repository}/
```

---

## 요구사항

- Rust 1.68 이상 (스파스 레지스트리 지원)
- Cargo (Rust 설치 시 자동 포함)

```bash
rustup update
rustc --version
# rustc 1.7x.x (...)
```

---

## 레지스트리 설정

### ~/.cargo/config.toml 수정

```toml
[registries.myregistry]
index = "sparse+http://server:9300/cargo/rustlib/"

# 기본 레지스트리는 crates.io 유지
[registry]
default = "crates-io"
```

!!! tip "레지스트리 이름"
    레지스트리 이름(`myregistry`)은 자유롭게 지정할 수 있습니다.
    miraeboy의 리포지토리 이름과 다를 수 있습니다.

### 프로젝트별 .cargo/config.toml

프로젝트 디렉터리에 `.cargo/config.toml`을 만들면 해당 프로젝트에만 적용됩니다.

```toml
# .cargo/config.toml (프로젝트 루트)
[registries.internal]
index = "sparse+http://miraeboy.example.com:9300/cargo/rustlib/"
```

---

## 인증 설정

### JWT 토큰으로 인증

miraeboy는 Cargo의 API 토큰 인증을 JWT로 처리합니다.

1. mboy로 로그인하여 토큰을 획득합니다.

```bash
mboy login --server http://server:9300 --user admin --password admin123
```

2. 토큰을 조회합니다.

```bash
mboy status --json | jq -r '.token'
```

3. Cargo 인증 토큰을 설정합니다.

```bash
cargo login --registry myregistry <위에서_조회한_JWT_토큰>
```

토큰은 `~/.cargo/credentials.toml`에 저장됩니다:

```toml
[registries.myregistry]
token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

!!! warning "토큰 유효 기간"
    JWT 토큰의 기본 유효 기간은 24시간입니다. 만료 시 `cargo login`으로 재설정하세요.

### CI/CD 환경에서 인증

환경 변수를 사용하거나 credentials.toml을 직접 설정합니다.

```bash
# 환경 변수 방식
CARGO_REGISTRIES_MYREGISTRY_TOKEN="your-jwt-token" cargo publish --registry myregistry
```

---

## 크레이트 배포 (publish)

### Cargo.toml 설정

```toml
[package]
name = "myutils"
version = "0.1.0"
edition = "2021"
description = "사내 공용 유틸리티 크레이트"
license = "MIT"
# 내부 레지스트리 배포 시 publish 필드 설정
publish = ["myregistry"]

[dependencies]
serde = { version = "1.0", features = ["derive"] }
```

### 크레이트 배포

```bash
# 레지스트리에 배포
cargo publish --registry myregistry

# 드라이 런 (실제 배포 없이 검증)
cargo publish --registry myregistry --dry-run
```

### 다른 크레이트에서 사용

```toml
# 다른 프로젝트의 Cargo.toml
[dependencies]
myutils = { version = "0.1.0", registry = "myregistry" }
```

---

## 크레이트 검색

### mboy CLI로 검색

```bash
# 전체 목록
mboy cargo search rustlib

# 이름으로 검색
mboy cargo search rustlib "myutils"
```

### Cargo 클라이언트로 검색

```bash
cargo search myutils --registry myregistry
```

---

## Yank / Unyank

특정 버전에 심각한 버그가 있을 때 yank로 새 다운로드를 차단합니다.
(이미 yank된 버전을 사용하는 기존 프로젝트는 영향 없음)

### Yank (버전 차단)

```bash
# mboy CLI
mboy cargo yank rustlib myutils 0.1.0

# Cargo 클라이언트
cargo yank --version 0.1.0 --registry myregistry myutils
```

### Unyank (차단 해제)

```bash
# mboy CLI
mboy cargo unyank rustlib myutils 0.1.0

# Cargo 클라이언트
cargo yank --undo --version 0.1.0 --registry myregistry myutils
```

---

## REST API

### 인덱스 조회

```http
GET /cargo/{repository}/config.json
```

```bash
curl http://server:9300/cargo/rustlib/config.json
```

응답:

```json
{
  "dl": "http://server:9300/cargo/rustlib/dl",
  "api": "http://server:9300/cargo/rustlib"
}
```

### 크레이트 인덱스 엔트리 조회

```http
GET /cargo/{repository}/{prefix}/{name}
Authorization: Bearer <token>
```

인덱스 파일은 줄바꿈으로 구분된 JSON 형식입니다.

```bash
# 예: myutils 크레이트의 인덱스
curl http://server:9300/cargo/rustlib/my/ut/myutils \
  -H "Authorization: Bearer $TOKEN"
```

### 크레이트 다운로드

```http
GET /cargo/{repository}/dl/{name}/{version}/download
Authorization: Bearer <token>
```

```bash
curl http://server:9300/cargo/rustlib/dl/myutils/0.1.0/download \
  -H "Authorization: Bearer $TOKEN" \
  -o myutils-0.1.0.crate
```

### 크레이트 검색

```http
GET /cargo/{repository}/api/v1/crates?q={query}
Authorization: Bearer <token>
```

```bash
curl "http://server:9300/cargo/rustlib/api/v1/crates?q=myutils" \
  -H "Authorization: Bearer $TOKEN"
```

응답:

```json
{
  "crates": [
    {
      "name": "myutils",
      "max_version": "0.1.0",
      "description": "사내 공용 유틸리티 크레이트"
    }
  ],
  "meta": {
    "total": 1
  }
}
```

### 크레이트 배포

```http
PUT /cargo/{repository}/api/v1/crates/new
Authorization: Bearer <token>
Content-Type: application/octet-stream
```

일반적으로 `cargo publish` 명령이 자동으로 처리합니다.

### Yank

```http
DELETE /cargo/{repository}/api/v1/crates/{name}/{version}/yank
Authorization: Bearer <token>
```

### Unyank

```http
PUT /cargo/{repository}/api/v1/crates/{name}/{version}/unyank
Authorization: Bearer <token>
```

---

## mboy init cargo — 프로젝트 초기화

새 Cargo 프로젝트 템플릿을 빠르게 생성합니다.

```bash
mboy init cargo \
  --dir ./myutils \
  --name myutils \
  --version 0.1.0 \
  --server http://miraeboy.example.com:9300 \
  --repo rustlib
```

생성되는 파일:

```
myutils/
├── Cargo.toml          ← publish 필드 포함
├── src/
│   └── lib.rs
└── .cargo/
    └── config.toml     ← 레지스트리 설정 포함
```

---

## 자주 발생하는 문제

### 인증 오류

```
error: failed to get `myutils` as a dependency of package `myapp`
  caused by: 401 Unauthorized
```

해결:

```bash
cargo login --registry myregistry <JWT_TOKEN>
```

### 레지스트리 URL 오류

```
error: registry `myregistry` not found in any configuration files
```

`~/.cargo/config.toml` 또는 프로젝트의 `.cargo/config.toml`에 레지스트리가 올바르게 등록되었는지 확인하세요.

### TLS 오류 (자체 서명 인증서)

```toml
# ~/.cargo/config.toml
[http]
cainfo = "/path/to/ca-cert.pem"   # 자체 CA 인증서 경로
```

또는 검증 비활성화 (개발 환경 전용):

```toml
[http]
check-revoke = false
```

### 버전 충돌

같은 크레이트의 여러 버전이 존재할 때 `Cargo.lock`이 업데이트되지 않으면 발생합니다.

```bash
cargo update -p myutils
```
