# Conan 레지스트리

miraeboy의 Conan v2 패키지 레지스트리 사용 방법을 설명합니다.

---

## 개요

miraeboy는 Conan v2 프로토콜을 구현한 패키지 레지스트리입니다.
기존 `conan` 클라이언트와 완전히 호환됩니다.

**기본 URL 패턴:**
```
http://server:9300/api/conan/{repository}
```

**Artifactory 호환 URL (활성화 시):**
```
http://server:9300/artifactory/api/conan/{repository}
```

---

## 요구사항

- Conan 2.x 이상 (`conan --version` 확인)

```bash
pip install conan --upgrade
conan --version
# conan version 2.x.x
```

---

## remote 등록

```bash
# 기본 등록
conan remote add extralib http://server:9300/api/conan/extralib

# 등록된 remote 확인
conan remote list
```

출력 예시:

```
conancenter: https://center2.conan.io [Verify SSL: True, Enabled: True]
extralib: http://server:9300/api/conan/extralib [Verify SSL: True, Enabled: True]
```

### 기존 remote 업데이트

```bash
conan remote update extralib --url http://new-server:9300/api/conan/extralib
```

### remote 비활성화/활성화

```bash
conan remote disable extralib
conan remote enable extralib
```

---

## 인증

### 로그인

```bash
conan remote login extralib <username>
# 비밀번호 프롬프트 표시
```

또는 비밀번호를 직접 지정:

```bash
conan remote login extralib admin -p admin123
```

### JWT 토큰 방식

Conan v2는 `/users/authenticate` 엔드포인트에서 Basic Auth로 JWT 토큰을 발급받아 사용합니다.
`conan remote login` 명령이 이 과정을 자동으로 처리합니다.

```bash
# 토큰 직접 발급 (스크립트 용도)
TOKEN=$(curl -s -u "admin:admin123" \
  http://server:9300/api/conan/extralib/v2/users/authenticate \
  | jq -r '.token')
```

### 로그아웃

```bash
conan remote logout extralib
```

---

## 패키지 빌드 및 업로드

### conanfile.py 작성 예시

```python
# conanfile.py
from conan import ConanFile
from conan.tools.cmake import CMakeToolchain, CMake, cmake_layout

class MylibConan(ConanFile):
    name = "mylib"
    version = "1.0.0"
    description = "사내 공용 유틸리티 라이브러리"
    license = "MIT"
    url = "https://github.com/mycompany/mylib"

    settings = "os", "compiler", "build_type", "arch"
    options = {"shared": [True, False], "fPIC": [True, False]}
    default_options = {"shared": False, "fPIC": True}

    exports_sources = "CMakeLists.txt", "src/*", "include/*"

    def layout(self):
        cmake_layout(self)

    def generate(self):
        tc = CMakeToolchain(self)
        tc.generate()

    def build(self):
        cmake = CMake(self)
        cmake.configure()
        cmake.build()

    def package(self):
        cmake = CMake(self)
        cmake.install()

    def package_info(self):
        self.cpp_info.libs = ["mylib"]
```

### 패키지 빌드

```bash
# 로컬 빌드
conan create . --user sc --channel dev

# 특정 설정으로 빌드
conan create . --user sc --channel dev \
  -s build_type=Release \
  -s compiler=gcc \
  -s compiler.version=12
```

### 레지스트리에 업로드

```bash
# 단일 패키지 업로드
conan upload "mylib/1.0.0@sc/dev" --remote extralib

# 확인 없이 업로드
conan upload "mylib/1.0.0@sc/dev" --remote extralib --confirm

# 모든 패키지 업로드 (레시피 + 바이너리 포함)
conan upload "*" --remote extralib --confirm
```

---

## 패키지 다운로드 및 사용

### 직접 설치

```bash
conan install --requires "mylib/1.0.0@sc/dev" --remote extralib
```

### conanfile.txt에서 사용

```ini
# conanfile.txt
[requires]
mylib/1.0.0@sc/dev
zlib/1.2.13

[generators]
CMakeDeps
CMakeToolchain
```

```bash
# 의존성 설치
conan install . --remote extralib
```

### CMakeLists.txt에서 사용

```cmake
cmake_minimum_required(VERSION 3.15)
project(myapp)

find_package(mylib REQUIRED)

add_executable(myapp main.cpp)
target_link_libraries(myapp mylib::mylib)
```

---

## 패키지 검색

### CLI로 검색

```bash
# 모든 패키지 검색
mboy package search extralib

# 이름으로 검색
mboy package search extralib "mylib*"
```

### conan 클라이언트로 검색

```bash
# 레지스트리에서 검색
conan search "mylib*" --remote extralib

# 특정 패키지 버전 목록
conan search "mylib" --remote extralib
```

출력 예시:

```
Existing packages for recipe mylib/1.0.0@sc/dev:

Package_ID: a1b2c3d4e5...
    [options]
        shared: False
    [settings]
        arch: x86_64
        build_type: Release
        compiler: gcc
        compiler.version: 12
        os: Linux
```

---

## 리비전 관리

Conan v2는 패키지 리비전(RREV, PREV)을 지원합니다.

### 리비전 목록 조회

```bash
# 레시피 리비전 목록
curl http://server:9300/api/conan/extralib/v2/conans/mylib/1.0.0/sc/dev/revisions \
  -H "Authorization: Bearer $TOKEN"
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

### 최신 리비전 조회

```bash
curl http://server:9300/api/conan/extralib/v2/conans/mylib/1.0.0/sc/dev/revisions/latest \
  -H "Authorization: Bearer $TOKEN"
```

### 특정 리비전 삭제 (delete 권한 필요)

```bash
# mboy 또는 conan CLI로 삭제
conan remove "mylib/1.0.0@sc/dev#abc123def456" --remote extralib --confirm
```

---

## Artifactory 호환 모드

기존에 Artifactory를 사용하던 팀이 miraeboy로 마이그레이션할 때 URL을 변경하지 않아도 됩니다.

config.yaml에서 활성화:

```yaml
server:
  artifactory_compat: true
```

활성화 후 다음 URL 패턴이 모두 동작합니다:

```bash
# 기본 URL
conan remote add extralib http://server:9300/api/conan/extralib

# Artifactory 호환 URL
conan remote add extralib http://server:9300/artifactory/api/conan/extralib
```

---

## mboy init conan — 프로젝트 초기화

새 Conan 프로젝트를 빠르게 시작할 수 있는 템플릿을 생성합니다.

```bash
mboy init conan \
  --dir ./mylib \
  --name mylib \
  --version 1.0.0 \
  --user sc \
  --channel dev \
  --server http://miraeboy.example.com:9300 \
  --repo extralib
```

생성되는 파일:

```
mylib/
├── conanfile.py       ← 기본 Conan 레시피
├── CMakeLists.txt     ← CMake 빌드 파일
├── src/
│   └── mylib.cpp
└── include/
    └── mylib.h
```

---

## 자주 발생하는 문제

### 인증 오류

```
ERROR: mylib/1.0.0@sc/dev: Remote 'extralib' login required.
```

해결:

```bash
conan remote login extralib <username> -p <password>
```

### 네임스페이스/채널 제한 오류

```
ERROR: Package namespace 'other' is not allowed in repository 'extralib'
```

리포지토리의 `allowed_namespaces` 설정을 확인하세요:

```bash
mboy repo get extralib
```

### SSL 인증서 오류 (자체 서명 인증서)

```bash
conan remote add extralib https://server:9300/api/conan/extralib --insecure
```

!!! warning "프로덕션에서 --insecure 사용 금지"
    개발 환경에서만 `--insecure` 옵션을 사용하세요.
    프로덕션에서는 유효한 TLS 인증서를 사용해야 합니다.
