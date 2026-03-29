# 빠른 시작 가이드

5분 만에 miraeboy를 설치하고 첫 번째 패키지를 등록하는 과정을 안내합니다.

---

## 사전 준비

- Linux 또는 macOS 환경
- curl 설치됨
- Conan 2.x 또는 Cargo 툴체인 (패키지 등록 시 필요)

---

## 1단계: 서버 설치 및 시작

```bash
# 서버 설치
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh

# 작업 디렉터리 생성
mkdir -p ~/miraeboy-data

# 최소 설정 파일 생성
cat > config.yaml << 'EOF'
server:
  address: ":9300"
  storage_path: "./data"
  node_role: "primary"

auth:
  jwt_secret: "change-me-in-production-use-random-string"
  users:
    - username: "admin"
      password: "admin123"
      admin: true

repositories: []
EOF

# 서버 시작 (백그라운드)
miraeboy --config config.yaml &
```

서버가 정상적으로 시작되었는지 확인합니다.

```bash
curl http://localhost:9300/ping
# 응답: {"status":"ok"}
```

---

## 2단계: CLI 설치 및 로그인

```bash
# mboy CLI 설치
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.sh | sh

# 서버에 로그인
mboy login --server http://localhost:9300 --user admin --password admin123
# 출력: 로그인 성공. 토큰이 저장되었습니다.

# 연결 상태 확인
mboy status
```

출력 예시:

```
서버: http://localhost:9300
사용자: admin (관리자)
버전: v1.0.0
상태: 정상
```

---

## 3단계: 리포지토리 생성

```bash
# Conan 패키지용 리포지토리 생성
mboy repo create \
  --name extralib \
  --owner admin \
  --desc "사내 C/C++ 외부 라이브러리"

# 생성 확인
mboy repo list
```

출력 예시:

```
NAME       OWNER   TYPE    설명
extralib   admin   conan   사내 C/C++ 외부 라이브러리
```

---

## 4단계: Conan 패키지 등록 및 사용

### Conan remote 등록

```bash
conan remote add extralib http://localhost:9300/api/conan/extralib
```

### 인증 설정

```bash
conan remote login extralib admin -p admin123
```

### 패키지 빌드 및 업로드

간단한 테스트용 패키지를 만들어 업로드해 보겠습니다.

```bash
# 테스트 패키지 디렉터리 생성
mkdir -p /tmp/hello-conan && cd /tmp/hello-conan

# conanfile.py 작성
cat > conanfile.py << 'EOF'
from conan import ConanFile

class HelloConan(ConanFile):
    name = "hello"
    version = "0.1.0"
    description = "테스트 패키지"
    package_type = "header-library"

    def package_info(self):
        self.cpp_info.bindirs = []
        self.cpp_info.libdirs = []
EOF

# 패키지 생성
conan create . --user sc --channel dev

# 레지스트리에 업로드
conan upload "hello/0.1.0@sc/dev" --remote extralib --confirm
```

### 패키지 검색 및 다운로드

```bash
# 레지스트리에서 검색
conan search "hello*" --remote extralib

# 패키지 다운로드 (다른 프로젝트에서)
conan install --requires "hello/0.1.0@sc/dev" --remote extralib
```

---

## 5단계: Cargo 레지스트리 사용 (선택)

Rust 크레이트를 등록하려면 별도의 Cargo용 리포지토리를 만듭니다.

```bash
# Cargo 레지스트리용 리포지토리 생성
mboy repo create --name rustlib --owner admin --desc "사내 Rust 크레이트"
```

`~/.cargo/config.toml`에 레지스트리를 등록합니다.

```toml
[registries.rustlib]
index = "sparse+http://localhost:9300/cargo/rustlib/"

[registry]
default = "crates-io"
```

Cargo 인증 토큰을 설정합니다.

```bash
# JWT 토큰 조회
mboy status --json | jq -r '.token'

# Cargo 인증 설정
cargo login --registry rustlib <위에서_조회한_토큰>
```

테스트 크레이트를 배포합니다.

```bash
mkdir -p /tmp/hello-rust && cd /tmp/hello-rust
cargo init --lib

# Cargo.toml 수정
cat > Cargo.toml << 'EOF'
[package]
name = "hello-rust"
version = "0.1.0"
edition = "2021"
description = "테스트 Rust 크레이트"
license = "MIT"
EOF

cargo publish --registry rustlib
```

---

## 다음 단계

!!! success "완료"
    miraeboy 서버가 정상 동작하고 첫 번째 패키지를 등록했습니다.

- [사용자 관리](user-management.md) — 팀원 계정 추가 및 권한 설정
- [리포지토리 관리](repository-management.md) — 리포지토리 세부 설정
- [Conan 레지스트리 가이드](conan-registry.md) — Conan 고급 사용법
- [Cargo 레지스트리 가이드](cargo-registry.md) — Cargo 고급 사용법
- [프로덕션 배포](../deploy/production.md) — nginx, TLS 등 운영 환경 구성
