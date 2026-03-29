# mboy CLI 명령어 레퍼런스

`mboy`는 miraeboy 서버를 관리하는 CLI 도구입니다.

---

## 설치 및 초기 설정

```bash
# 설치
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.sh | sh

# 로그인
mboy login --server http://localhost:9300 --user admin --password admin123
```

---

## 글로벌 플래그

모든 명령에서 사용할 수 있는 플래그입니다.

| 플래그 | 약어 | 설명 | 기본값 |
|--------|------|------|--------|
| `--server URL` | — | 서버 주소 | `http://localhost:9300` |
| `--token TOKEN` | — | API 토큰 (로그인 대신 직접 지정) | — |
| `--json` | — | 출력을 JSON 형식으로 | `false` |
| `--version` | — | 버전 정보 출력 | — |

### 환경 변수

| 환경 변수 | 설명 |
|-----------|------|
| `MBOY_SERVER` | 서버 주소 기본값 |
| `MBOY_TOKEN` | API 토큰 기본값 |

```bash
export MBOY_SERVER=http://miraeboy.example.com:9300
export MBOY_TOKEN=your-jwt-token
```

---

## mboy login

서버에 로그인하고 JWT 토큰을 저장합니다.

```
mboy login [--server URL] [--user USERNAME] [--password PASSWORD]
```

| 플래그 | 설명 |
|--------|------|
| `--server URL` | 서버 주소 |
| `--user USERNAME` | 사용자명 |
| `--password PASSWORD` | 비밀번호 (생략 시 프롬프트 표시) |

**예시:**

```bash
# 대화형 로그인
mboy login

# 플래그 지정
mboy login --server http://server:9300 --user admin --password admin123
```

토큰은 `~/.mboy/config.json`에 저장됩니다.

---

## mboy status

현재 서버 연결 상태와 로그인 정보를 표시합니다.

```
mboy status
```

**예시:**

```bash
mboy status
# 서버: http://server:9300
# 사용자: admin (관리자)
# 버전: v1.0.0
# 상태: 정상

mboy status --json
```

---

## mboy version

CLI 버전을 출력합니다.

```
mboy version
```

---

## mboy user

### mboy user list

사용자 목록을 조회합니다. (관리자 전용)

```
mboy user list [--json]
```

**예시:**

```bash
mboy user list
mboy user list --json
```

### mboy user create

새 사용자를 생성합니다. (관리자 전용)

```
mboy user create --username USERNAME --password PASSWORD [--admin]
```

| 플래그 | 필수 | 설명 |
|--------|------|------|
| `--username USERNAME` | ✓ | 사용자명 |
| `--password PASSWORD` | ✓ | 비밀번호 |
| `--admin` | — | 관리자 권한 부여 |

**예시:**

```bash
mboy user create --username dev2 --password "Pass123!"
mboy user create --username ops --password "OpsPass456!" --admin
```

### mboy user update

사용자 정보를 수정합니다. (관리자 전용)

```
mboy user update <username> [--password PASSWORD] [--admin true|false]
```

| 인자/플래그 | 설명 |
|------------|------|
| `<username>` | 수정할 사용자명 |
| `--password PASSWORD` | 새 비밀번호 |
| `--admin true\|false` | 관리자 권한 설정 |

**예시:**

```bash
mboy user update dev2 --password "NewPass789!"
mboy user update dev2 --admin true
mboy user update dev2 --admin false
```

### mboy user delete

사용자를 삭제합니다. (관리자 전용)

```
mboy user delete <username>
```

**예시:**

```bash
mboy user delete dev2
```

---

## mboy repo

### mboy repo list

리포지토리 목록을 조회합니다.

```
mboy repo list [--json]
```

### mboy repo create

리포지토리를 생성합니다. (관리자 전용)

```
mboy repo create --name NAME --owner OWNER [옵션...]
```

| 플래그 | 필수 | 설명 |
|--------|------|------|
| `--name NAME` | ✓ | 리포지토리 이름 |
| `--owner OWNER` | ✓ | 소유자 사용자명 |
| `--desc DESC` | — | 설명 |
| `--anon read\|write\|none` | — | 익명 접근 권한 (기본: `none`) |
| `--ns NS1,NS2` | — | 허용 네임스페이스 (콤마 구분) |
| `--ch CH1,CH2` | — | 허용 채널 (콤마 구분) |
| `--git-url URL` | — | Git 레시피 동기화 URL |
| `--git-branch BRANCH` | — | Git 브랜치 (기본: `main`) |
| `--git-token TOKEN` | — | Git 인증 토큰 |

**예시:**

```bash
# 기본 생성
mboy repo create --name extralib --owner dev1

# 전체 옵션
mboy repo create \
  --name extralib \
  --owner dev1 \
  --desc "사내 외부 라이브러리" \
  --anon none \
  --ns sc,mycompany \
  --ch dev,release \
  --git-url https://github.com/corp/recipes.git \
  --git-branch main \
  --git-token ghp_xxxx
```

### mboy repo get

리포지토리 상세 정보를 조회합니다.

```
mboy repo get <name> [--json]
```

**예시:**

```bash
mboy repo get extralib
mboy repo get extralib --json
```

### mboy repo update

리포지토리 설정을 수정합니다. (소유자 또는 관리자)

```
mboy repo update <name> [옵션...]
```

| 플래그 | 설명 |
|--------|------|
| `--desc DESC` | 설명 변경 |
| `--anon read\|write\|none` | 익명 접근 권한 변경 |
| `--ns NS1,NS2` | 허용 네임스페이스 변경 |
| `--ch CH1,CH2` | 허용 채널 변경 |

**예시:**

```bash
mboy repo update extralib --desc "새 설명"
mboy repo update extralib --anon read
mboy repo update extralib --ch dev,release,stable
```

### mboy repo delete

리포지토리를 삭제합니다. (관리자 전용)

```
mboy repo delete <name> [--force]
```

| 플래그 | 설명 |
|--------|------|
| `--force` | 패키지가 있어도 강제 삭제 |

**예시:**

```bash
mboy repo delete extralib
mboy repo delete extralib --force
```

---

## mboy member

### mboy member list

리포지토리 멤버 목록을 조회합니다.

```
mboy member list <repo> [--json]
```

**예시:**

```bash
mboy member list extralib
```

### mboy member add

멤버를 추가합니다.

```
mboy member add <repo> <username> <permission>
```

권한: `read`, `write`, `delete`, `owner`

**예시:**

```bash
mboy member add extralib dev2 read
mboy member add extralib ci write
mboy member add extralib senior delete
mboy member add extralib teamlead owner
```

### mboy member update

멤버 권한을 변경합니다.

```
mboy member update <repo> <username> <permission>
```

**예시:**

```bash
mboy member update extralib dev2 write
```

### mboy member remove

멤버를 제거합니다.

```
mboy member remove <repo> <username>
```

**예시:**

```bash
mboy member remove extralib dev2
```

---

## mboy package

### mboy package search

Conan 패키지를 검색합니다.

```
mboy package search <repo> [query] [--json]
```

**예시:**

```bash
# 전체 목록
mboy package search extralib

# 검색어로 필터
mboy package search extralib "mylib*"
```

---

## mboy cargo

### mboy cargo search

Cargo 크레이트를 검색합니다.

```
mboy cargo search <repo> [query] [--json]
```

**예시:**

```bash
mboy cargo search rustlib
mboy cargo search rustlib "myutils*"
```

### mboy cargo yank

크레이트 특정 버전을 yank합니다 (새 다운로드 차단).

```
mboy cargo yank <repo> <name> <version>
```

**예시:**

```bash
mboy cargo yank rustlib myutils 0.1.0
```

### mboy cargo unyank

yank된 크레이트 버전을 복구합니다.

```
mboy cargo unyank <repo> <name> <version>
```

**예시:**

```bash
mboy cargo unyank rustlib myutils 0.1.0
```

---

## mboy build

### mboy build list

빌드 목록을 조회합니다.

```
mboy build list [--json]
```

**예시:**

```bash
mboy build list
mboy build list --json
```

### mboy build trigger

빌드를 트리거합니다.

```
mboy build trigger --repo NAME [--git-url URL] [--ref REF] [--platforms PLATFORMS]
```

| 플래그 | 필수 | 설명 |
|--------|------|------|
| `--repo NAME` | ✓ | 빌드 대상 리포지토리 |
| `--git-url URL` | — | Git 소스 URL |
| `--ref REF` | — | Git 브랜치, 태그, 커밋 해시 |
| `--platforms PLATFORMS` | — | 빌드 플랫폼 목록 (콤마 구분) |

**예시:**

```bash
# 기본 트리거
mboy build trigger --repo extralib

# Git 소스 지정
mboy build trigger \
  --repo extralib \
  --git-url https://github.com/mycompany/mylib.git \
  --ref v1.2.0

# 다중 플랫폼
mboy build trigger \
  --repo extralib \
  --git-url https://github.com/mycompany/mylib.git \
  --ref main \
  --platforms linux/amd64,windows/amd64,darwin/arm64
```

### mboy build get

빌드 상세 정보를 조회합니다.

```
mboy build get <id> [--json]
```

**예시:**

```bash
mboy build get 42
mboy build get 42 --json
```

---

## mboy init

프로젝트 초기화 템플릿을 생성합니다.

### mboy init conan

Conan 프로젝트 템플릿을 생성합니다.

```
mboy init conan [옵션...]
```

| 플래그 | 설명 | 기본값 |
|--------|------|--------|
| `--dir DIR` | 생성 디렉터리 | 현재 디렉터리 |
| `--name NAME` | 패키지 이름 | — |
| `--version VER` | 버전 | `0.1.0` |
| `--user USER` | Conan 사용자(네임스페이스) | — |
| `--channel CH` | Conan 채널 | `dev` |
| `--server URL` | 서버 주소 | — |
| `--repo REPO` | 리포지토리 이름 | — |

**예시:**

```bash
mboy init conan \
  --dir ./mylib \
  --name mylib \
  --version 1.0.0 \
  --user sc \
  --channel dev \
  --server http://server:9300 \
  --repo extralib
```

### mboy init cargo

Cargo 프로젝트 템플릿을 생성합니다.

```
mboy init cargo [옵션...]
```

| 플래그 | 설명 | 기본값 |
|--------|------|--------|
| `--dir DIR` | 생성 디렉터리 | 현재 디렉터리 |
| `--name NAME` | 크레이트 이름 | — |
| `--version VER` | 버전 | `0.1.0` |
| `--server URL` | 서버 주소 | — |
| `--repo REPO` | 리포지토리 이름 | — |

**예시:**

```bash
mboy init cargo \
  --dir ./myutils \
  --name myutils \
  --version 0.1.0 \
  --server http://server:9300 \
  --repo rustlib
```

### mboy init team

팀 설정 파일 템플릿을 생성합니다.

```
mboy init team [--dir DIR] [--server URL] [--team TEAM]
```

**예시:**

```bash
mboy init team \
  --dir ./team-config \
  --server http://server:9300 \
  --team backend-team
```

---

## mboy completion

셸 자동 완성 스크립트를 출력합니다.

```
mboy completion <shell>
```

지원 셸: `bash`, `zsh`, `fish`

**예시:**

```bash
# bash
mboy completion bash > /etc/bash_completion.d/mboy

# zsh (Oh My Zsh)
mboy completion zsh > "${fpath[1]}/_mboy"

# fish
mboy completion fish > ~/.config/fish/completions/mboy.fish
```

---

## 출력 포맷

`--json` 플래그를 사용하면 모든 명령의 출력이 JSON으로 변환됩니다. 스크립트나 자동화 작업에 유용합니다.

```bash
# JSON 출력을 jq로 처리
mboy repo list --json | jq '.[].name'
mboy build list --json | jq '.[] | select(.status == "failed")'
mboy user list --json | jq '.[] | select(.admin == true) | .username'
```
