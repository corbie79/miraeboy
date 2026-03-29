package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CmdInit generates project config templates or team setup files.
//
//	mboy init conan   [--dir DIR] [--name NAME] [--version VER] [--user USER] [--channel CH]
//	mboy init cargo   [--dir DIR] [--name NAME] [--version VER]
//	mboy init team    [--server URL] [--repo NAME]
func CmdInit(kind string, args []string) error {
	switch kind {
	case "conan":
		return initConan(args)
	case "cargo":
		return initCargo(args)
	case "team":
		return initTeam(args)
	default:
		return fmt.Errorf("알 수 없는 템플릿: %q\n사용 가능: conan | cargo | team", kind)
	}
}

// ─── conan ────────────────────────────────────────────────────────────────────

func initConan(args []string) error {
	fs := flag.NewFlagSet("init conan", flag.ContinueOnError)
	dir     := fs.String("dir", ".", "출력 디렉토리")
	name    := fs.String("name", "mylib", "라이브러리 이름")
	version := fs.String("version", "1.0.0", "버전")
	user    := fs.String("user", "mycompany", "Conan 유저(네임스페이스)")
	channel := fs.String("channel", "stable", "채널")
	server  := fs.String("server", "http://miraeboy.example.com:9300", "miraeboy 서버 URL")
	repo    := fs.String("repo", "conan-local", "Conan 리포지토리 이름")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}

	files := map[string]string{
		"conanfile.py":         conanfilePy(*name, *version, *user, *channel),
		"conanprofile.ini":     conanProfile(*server, *repo),
		".conan/remotes.json":  conanRemotes(*server, *repo),
		"CMakeLists.txt":       cmakeLists(*name, *version),
		"cmake/conan.cmake":    conanCmake(),
	}

	for path, content := range files {
		full := filepath.Join(*dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := writeFileIfNotExists(full, content); err != nil {
			return err
		}
		fmt.Printf("  생성: %s\n", path)
	}

	fmt.Printf(`
Conan 프로젝트 템플릿 생성 완료!

다음 단계:
  1. conan profile show default           # 기본 프로파일 확인
  2. conan remote add miraeboy %s/api/conan/%s  # 리모트 등록
  3. conan create . %s/%s@%s/%s           # 패키지 빌드 & 업로드
`, *server, *repo, *name, *version, *user, *channel)
	return nil
}

func conanfilePy(name, version, user, channel string) string {
	return fmt.Sprintf(`from conan import ConanFile
from conan.tools.cmake import CMakeToolchain, CMake, cmake_layout


class %sConan(ConanFile):
    name = "%s"
    version = "%s"
    # 메타데이터
    license = "MIT"
    author  = "%s"
    description = "%s 라이브러리"
    topics  = ()

    # 빌드 설정
    settings = "os", "compiler", "build_type", "arch"
    options  = {"shared": [True, False], "fPIC": [True, False]}
    default_options = {"shared": False, "fPIC": True}

    # 소스
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
        self.cpp_info.libs = ["%s"]
`, capitalize(name), name, version, user, name, name)
}

func conanProfile(server, repo string) string {
	return fmt.Sprintf(`# Conan 프로파일 — miraeboy 서버 설정
# 사용법: conan install . -pr conanprofile.ini

[settings]
os=Linux
arch=x86_64
compiler=gcc
compiler.version=13
compiler.libcxx=libstdc++11
build_type=Release

[conf]
tools.system.package_manager:mode=install
tools.system.package_manager:sudo=True

[tool_requires]
cmake/[>=3.25]

# miraeboy 리모트 등록 (최초 1회)
# conan remote add miraeboy %s/api/conan/%s
`, server, repo)
}

func conanRemotes(server, repo string) string {
	return fmt.Sprintf(`{
  "remotes": [
    {
      "name": "miraeboy",
      "url": "%s/api/conan/%s",
      "verify_ssl": true
    },
    {
      "name": "conancenter",
      "url": "https://center2.conan.io",
      "verify_ssl": true
    }
  ]
}
`, server, repo)
}

func cmakeLists(name, version string) string {
	vparts := strings.SplitN(version, ".", 3)
	for len(vparts) < 3 {
		vparts = append(vparts, "0")
	}
	return fmt.Sprintf(`cmake_minimum_required(VERSION 3.25)
project(%s VERSION %s LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Conan 의존성
find_package(Conan QUIET)

# 라이브러리
add_library(%s src/%s.cpp)
target_include_directories(%s PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)

# 설치
include(GNUInstallDirs)
install(TARGETS %s
    EXPORT %sTargets
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR}
    INCLUDES DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}
)
install(DIRECTORY include/ DESTINATION ${CMAKE_INSTALL_INCLUDEDIR})
`, name, version, name, name, name, name, capitalize(name))
}

func conanCmake() string {
	return `# conan_provider.cmake — Conan v2 CMake 통합
# https://github.com/conan-io/cmake-conan

cmake_minimum_required(VERSION 3.24)

macro(conan_provide_dependency package_name)
    set_property(GLOBAL PROPERTY CONAN_PROVIDE_DEPENDENCY_HANDLED TRUE)
    get_property(CONAN_INSTALL_ARGS GLOBAL PROPERTY CONAN_INSTALL_ARGS)
    string(REPLACE ";" "\n" CONAN_INSTALL_ARGS_STR "${CONAN_INSTALL_ARGS}")
    message(STATUS "Conan providing ${package_name}...")
endmacro()
`
}

// ─── cargo ────────────────────────────────────────────────────────────────────

func initCargo(args []string) error {
	fs := flag.NewFlagSet("init cargo", flag.ContinueOnError)
	dir     := fs.String("dir", ".", "출력 디렉토리")
	name    := fs.String("name", "my-crate", "크레이트 이름")
	version := fs.String("version", "0.1.0", "버전")
	server  := fs.String("server", "http://miraeboy.example.com:9300", "miraeboy 서버 URL")
	repo    := fs.String("repo", "cargo-local", "Cargo 리포지토리 이름")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}

	files := map[string]string{
		"Cargo.toml":             cargoToml(*name, *version, *repo),
		"src/lib.rs":             cargoLibRs(*name),
		".cargo/config.toml":     cargoConfig(*server, *repo),
		".github/workflows/publish.yml": cargoPublishWorkflow(*server, *repo),
	}

	for path, content := range files {
		full := filepath.Join(*dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := writeFileIfNotExists(full, content); err != nil {
			return err
		}
		fmt.Printf("  생성: %s\n", path)
	}

	fmt.Printf(`
Cargo 프로젝트 템플릿 생성 완료!

다음 단계:
  1. mboy login --server %s          # 토큰 발급
  2. 토큰을 ~/.cargo/credentials.toml 에 추가:
       [registries.miraeboy]
       token = "YOUR_TOKEN"
  3. cargo publish --registry miraeboy  # 퍼블리시
`, *server)
	return nil
}

func cargoToml(name, version, repo string) string {
	return fmt.Sprintf(`[package]
name = "%s"
version = "%s"
edition = "2021"
description = ""
license = "MIT"

# 의존성 예시 (miraeboy 레지스트리 + crates.io 혼합)
[dependencies]
# my-internal-crate = { version = "1.0", registry = "miraeboy" }
# serde = { version = "1", features = ["derive"] }

[dev-dependencies]

[lib]
name = "%s"
crate-type = ["lib"]

# miraeboy 레지스트리로 퍼블리시
[package.metadata.publish]
registries = ["%s"]
`, name, version, strings.ReplaceAll(name, "-", "_"), repo)
}

func cargoLibRs(name string) string {
	return fmt.Sprintf(`//! %s crate

pub fn hello() -> &'static str {
    "Hello from %s!"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn it_works() {
        assert_eq!(hello(), "Hello from %s!");
    }
}
`, name, name, name)
}

func cargoConfig(server, repo string) string {
	return fmt.Sprintf(`# .cargo/config.toml — 팀 공용 Cargo 설정
# 이 파일을 프로젝트에 커밋하면 팀원 모두가 동일한 레지스트리를 사용합니다.

[registries]
# miraeboy 사내 Cargo 레지스트리
%s = { index = "sparse+%s/cargo/%s/" }

[net]
retry = 3

# 개발자별 토큰은 ~/.cargo/credentials.toml 에 저장 (커밋 금지!)
# [registries.%s]
# token = "YOUR_TOKEN_HERE"
`, repo, server, repo, repo)
}

func cargoPublishWorkflow(server, repo string) string {
	return fmt.Sprintf(`name: Publish to miraeboy

on:
  push:
    tags: ['v*']

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-rust@v1
        with:
          toolchain: stable

      - name: Configure registry
        run: |
          cat >> ~/.cargo/config.toml <<EOF
          [registries]
          %s = { index = "sparse+%s/cargo/%s/" }
          EOF

      - name: Publish
        env:
          CARGO_REGISTRIES_%s_TOKEN: ${{ secrets.MIRAEBOY_TOKEN }}
        run: cargo publish --registry %s
`, repo, server, repo, strings.ToUpper(repo), repo)
}

// ─── team ─────────────────────────────────────────────────────────────────────

func initTeam(args []string) error {
	fs := flag.NewFlagSet("init team", flag.ContinueOnError)
	dir    := fs.String("dir", ".", "출력 디렉토리")
	server := fs.String("server", "http://miraeboy.example.com:9300", "miraeboy 서버 URL")
	team   := fs.String("team", "myteam", "팀/조직 이름")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}

	files := map[string]string{
		"miraeboy-team.md":              teamReadme(*server, *team),
		"conan/remotes.json":            conanRemotes(*server, *team+"-conan"),
		"conan/profile-linux.ini":       conanProfileLinux(*server, *team+"-conan"),
		"conan/profile-macos.ini":       conanProfileMacos(*server, *team+"-conan"),
		"conan/profile-windows.ini":     conanProfileWindows(*server, *team+"-conan"),
		"cargo/config.toml":             cargoConfig(*server, *team+"-cargo"),
		"scripts/setup-dev-linux.sh":    setupDevLinux(*server),
		"scripts/setup-dev-macos.sh":    setupDevMacos(*server),
		"scripts/setup-dev-windows.ps1": setupDevWindows(*server),
	}

	for path, content := range files {
		full := filepath.Join(*dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := writeFileIfNotExists(full, content); err != nil {
			return err
		}
		fmt.Printf("  생성: %s\n", path)
	}

	fmt.Printf(`
팀 설정 템플릿 생성 완료!

이 디렉토리를 팀 위키 또는 별도 레포지토리에 공유하세요.
개발자 온보딩 절차는 miraeboy-team.md 를 참고하세요.
`)
	return nil
}

func teamReadme(server, team string) string {
	return fmt.Sprintf(`# %s 개발 환경 설정 가이드

## 서버 정보

- **miraeboy 서버**: %s
- **Conan 리포지토리**: %s-conan
- **Cargo 리포지토리**: %s-cargo

---

## 1. 개발 도구 설치

### Linux / macOS
`+"```"+`bash
curl -fsSL %s/install-agent.sh | sh --skip-devtools
# 빌드 도구 포함:
# curl -fsSL %s/install-agent.sh | sh
`+"```"+`

### Windows (PowerShell 관리자)
`+"```"+`powershell
irm %s/install-agent.ps1 | iex
`+"```"+`

---

## 2. CLI 도구 (mboy) 설치

`+"```"+`bash
# Linux / macOS
curl -fsSL %s/install-cli.sh | sh

# Windows
irm %s/install-cli.ps1 | iex
`+"```"+`

---

## 3. 로그인

`+"```"+`bash
mboy login --server %s
`+"```"+`

---

## 4. Conan 설정

`+"```"+`bash
conan remote add miraeboy %s/api/conan/%s-conan
conan remote login miraeboy <username>
`+"```"+`

프로파일 복사:
- Linux: `+"`conan/profile-linux.ini`"+`
- macOS: `+"`conan/profile-macos.ini`"+`
- Windows: `+"`conan/profile-windows.ini`"+`

---

## 5. Cargo 설정

`+"`cargo/config.toml`"+` 내용을 `+"`~/.cargo/config.toml`"+` 에 추가하거나,
프로젝트 루트의 `+"``.cargo/config.toml``"+` 에 복사합니다.

토큰 발급 후 `+"`~/.cargo/credentials.toml`"+` 에 추가:
`+"```"+`toml
[registries.%s-cargo]
token = "mboy login 후 발급된 토큰"
`+"```"+`
`, team, server, team, team,
		server, server, server,
		server, server,
		server,
		server, team,
		team)
}

func conanProfileLinux(server, repo string) string {
	return fmt.Sprintf(`# Conan 프로파일 — Linux
# 사용법: conan install . -pr conan/profile-linux.ini

[settings]
os=Linux
arch=x86_64
compiler=gcc
compiler.version=13
compiler.libcxx=libstdc++11
build_type=Release

[conf]
tools.system.package_manager:mode=install
tools.system.package_manager:sudo=True

# miraeboy 리모트: %s/api/conan/%s
`, server, repo)
}

func conanProfileMacos(server, repo string) string {
	return fmt.Sprintf(`# Conan 프로파일 — macOS (Apple Silicon)
# 사용법: conan install . -pr conan/profile-macos.ini

[settings]
os=Macos
arch=armv8
compiler=apple-clang
compiler.version=15
compiler.libcxx=libc++
compiler.cppstd=17
build_type=Release

# miraeboy 리모트: %s/api/conan/%s
`, server, repo)
}

func conanProfileWindows(server, repo string) string {
	return fmt.Sprintf(`# Conan 프로파일 — Windows (MSVC)
# 사용법: conan install . -pr conan/profile-windows.ini

[settings]
os=Windows
arch=x86_64
compiler=msvc
compiler.version=193
compiler.runtime=dynamic
compiler.cppstd=17
build_type=Release

# miraeboy 리모트: %s/api/conan/%s
`, server, repo)
}

func setupDevLinux(server string) string {
	return fmt.Sprintf(`#!/usr/bin/env sh
# Linux 개발 환경 설정 스크립트
set -e

# mboy CLI
curl -fsSL %s/raw/main/install-cli.sh | sh

# Conan 설치
pip3 install --user conan

# miraeboy 리모트 등록
conan remote add miraeboy %s/api/conan/myteam-conan 2>/dev/null || true
conan remote login miraeboy "$(read -p 'Username: ' u; echo $u)"

echo "완료! 개발 환경 설정이 끝났습니다."
`, server, server)
}

func setupDevMacos(server string) string {
	return fmt.Sprintf(`#!/usr/bin/env sh
# macOS 개발 환경 설정 스크립트
set -e

# Homebrew 없으면 설치
if ! command -v brew >/dev/null 2>&1; then
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# mboy CLI
curl -fsSL %s/raw/main/install-cli.sh | sh

# Python + Conan
brew install python3
pip3 install conan

# miraeboy 리모트 등록
conan remote add miraeboy %s/api/conan/myteam-conan 2>/dev/null || true

echo "완료! 개발 환경 설정이 끝났습니다."
`, server, server)
}

func setupDevWindows(server string) string {
	return fmt.Sprintf(`# Windows 개발 환경 설정 스크립트 (PowerShell)
# 관리자 권한으로 실행하세요

# mboy CLI
irm %s/raw/main/install-cli.ps1 | iex

# Python + Conan
if (-not (Get-Command python -ErrorAction SilentlyContinue)) {
    scoop install python
}
pip install conan

# miraeboy 리모트 등록
conan remote add miraeboy %s/api/conan/myteam-conan

Write-Host "완료! 개발 환경 설정이 끝났습니다." -ForegroundColor Green
`, server, server)
}

// ─── 공통 헬퍼 ────────────────────────────────────────────────────────────────

func writeFileIfNotExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  건너뜀 (이미 존재): %s\n", path)
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
