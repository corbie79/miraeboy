# miraeboy

**miraeboy**는 Go로 작성된 사내 패키지 레지스트리 서버로, Conan v2 (C/C++) 패키지와 Cargo (Rust) 크레이트를 단일 서버에서 관리합니다.

---

## 주요 기능

| 기능 | 설명 |
|------|------|
| **Conan v2 레지스트리** | C/C++ 패키지를 팀 내부에서 호스팅 |
| **Cargo 스파스 레지스트리** | Rust 크레이트를 내부 레지스트리에서 관리 |
| **리포지토리 관리** | 여러 리포지토리를 독립적으로 운영, 권한 세분화 |
| **사용자 관리** | 로컬 계정 + OIDC SSO (Keycloak, Azure AD 등) |
| **빌드 시스템** | 서버-에이전트 구조로 자동화 빌드 실행 |
| **Git 레시피 동기화** | Conan 레시피 파일을 Git 저장소에 자동 푸시 |
| **S3 호환 스토리지** | MinIO, AWS S3, 기타 S3 호환 스토리지 지원 |
| **복제 노드** | primary/replica 노드 구조로 고가용성 지원 |
| **Artifactory 호환** | `/artifactory/api/conan/...` URL 패턴 지원 |
| **JWT 인증** | 24시간 TTL JWT 기반 인증 |

---

## 아키텍처

```
                          ┌─────────────────────────────────┐
                          │          miraeboy 서버           │
                          │           (포트 9300)            │
                          │                                 │
  conan remote ──────────►│  /api/conan/{repo}/v2/          │
  cargo registry ────────►│  /cargo/{repo}/                 │
  mboy CLI ──────────────►│  /api/...                       │
  Web Browser ───────────►│  /api/auth/...                  │
                          │                                 │
                          │  ┌──────────┐  ┌─────────────┐ │
                          │  │  S3/MinIO│  │  Git 저장소  │ │
                          │  │ 스토리지 │  │  (레시피)   │ │
                          │  └──────────┘  └─────────────┘ │
                          └──────────────┬──────────────────┘
                                         │  빌드 트리거
                                         ▼
                          ┌─────────────────────────────────┐
                          │       miraeboy-agent             │
                          │      (빌드 워커)                  │
                          │                                 │
                          │  Conan 빌드 + 업로드            │
                          │  Cargo 빌드 + 업로드            │
                          └─────────────────────────────────┘
```

### 구성 요소

- **`miraeboy`** — 메인 서버 바이너리 (포트 9300)
- **`miraeboy-agent`** — 빌드 워커. 서버로부터 빌드 작업을 받아 실행
- **`mboy`** — 관리 CLI 도구. 서버 API를 편리하게 조작

---

## 빠른 시작 (3단계)

### 1단계: 서버 설치 및 실행

```bash
# 설치 스크립트로 한 번에 설치
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh

# 서버 실행
miraeboy --config config.yaml
```

### 2단계: CLI 설치 및 로그인

```bash
# CLI 설치
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.sh | sh

# 로그인
mboy login --server http://localhost:9300 --user admin --password admin123
```

### 3단계: 리포지토리 생성 및 패키지 사용

```bash
# 리포지토리 생성
mboy repo create --name mylib --owner admin

# Conan remote 등록
conan remote add mylib http://localhost:9300/api/conan/mylib

# Cargo 레지스트리 설정 (~/.cargo/config.toml)
cat >> ~/.cargo/config.toml << 'EOF'
[registries.mylib]
index = "sparse+http://localhost:9300/cargo/mylib/"
EOF
```

!!! tip "자세한 시작 가이드"
    처음 사용하신다면 [빠른 시작 가이드](guide/quickstart.md)를 참고하세요.

---

## 권한 체계

miraeboy는 5단계 권한을 사용합니다 (낮음 → 높음):

```
none  <  read  <  write  <  delete  <  owner
```

| 권한 | 허용 작업 |
|------|-----------|
| `none` | 접근 불가 |
| `read` | 패키지 다운로드만 가능 |
| `write` | 업로드 + 다운로드 |
| `delete` | 업로드 + 다운로드 + 패키지 삭제 |
| `owner` | 모든 작업 + 리포지토리 설정 및 멤버 관리 |
| `admin` (전역) | 모든 리포지토리에 대한 전체 권한 |

---

## 지원 URL 패턴

| 용도 | URL 패턴 |
|------|----------|
| Conan v2 레지스트리 | `http://server:9300/api/conan/{repository}` |
| Cargo 스파스 레지스트리 | `sparse+http://server:9300/cargo/{repository}/` |
| Artifactory 호환 | `http://server:9300/artifactory/api/conan/{repository}` |

---

## 문서 둘러보기

- [**설치 가이드**](installation/server.md) — 서버, 에이전트, CLI 설치 방법
- [**빠른 시작**](guide/quickstart.md) — 5분 만에 시작하기
- [**config.yaml 레퍼런스**](configuration/config-yaml.md) — 전체 설정 항목 설명
- [**REST API**](api/rest-api.md) — API 엔드포인트 레퍼런스
- [**mboy CLI**](cli/reference.md) — CLI 명령어 전체 레퍼런스
- [**프로덕션 배포**](deploy/production.md) — nginx, TLS, Docker 배포 가이드
