# Git 레시피 동기화

miraeboy의 Git 레시피 동기화 기능을 설명합니다.
Conan 패키지가 업로드될 때마다 레시피 파일이 자동으로 지정된 Git 저장소에 푸시됩니다.

---

## 개요

Git 레시피 동기화는 다음과 같이 동작합니다.

```
개발자  ──► conan upload mylib/1.0.0@sc/dev --remote extralib
                                │
                                ▼
                        miraeboy 서버
                        패키지 저장 + 레시피 추출
                                │
                                ▼
                        GitHub / GitLab / Gitea
                        (지정된 브랜치에 자동 커밋/푸시)
                                │
                                ▼
                        conan-recipes.git
                        sc/mylib/1.0.0/dev/
                        └── conanfile.py
```

**장점:**
- Conan 레시피를 Git으로 이력 관리
- 팀 전체가 레시피 변경 이력을 PR/MR로 리뷰 가능
- 레지스트리와 Git 간 레시피 자동 동기화

---

## 설정 방법

### 리포지토리 생성 시 Git 동기화 설정

```bash
mboy repo create \
  --name extralib \
  --owner dev1 \
  --git-url https://github.com/mycompany/conan-recipes.git \
  --git-branch main \
  --git-token ghp_xxxxxxxxxxxxxxxxxxxxxxxx
```

### 기존 리포지토리에 Git 동기화 추가

```bash
mboy repo update extralib \
  --git-url https://github.com/mycompany/conan-recipes.git \
  --git-branch main \
  --git-token ghp_xxxxxxxxxxxxxxxxxxxxxxxx
```

### config.yaml로 설정

```yaml
repositories:
  - name: "extralib"
    description: "사내 외부 라이브러리"
    owner: "dev1"
    git:
      url: "https://github.com/mycompany/conan-recipes.git"
      branch: "main"
      token: "ghp_xxxx"
```

---

## Git 저장소 구조

miraeboy는 다음 디렉터리 구조로 레시피를 푸시합니다.

```
conan-recipes/
└── {namespace}/
    └── {name}/
        └── {version}/
            └── {channel}/
                └── conanfile.py
```

예시:

```
conan-recipes/
├── sc/
│   ├── mylib/
│   │   ├── 1.0.0/
│   │   │   ├── dev/
│   │   │   │   └── conanfile.py
│   │   │   └── release/
│   │   │       └── conanfile.py
│   │   └── 1.1.0/
│   │       └── dev/
│   │           └── conanfile.py
│   └── anotherlib/
│       └── 2.0.0/
│           └── stable/
│               └── conanfile.py
└── mycompany/
    └── utils/
        └── 0.5.0/
            └── dev/
                └── conanfile.py
```

---

## 커밋 메시지 형식

자동 생성되는 커밋 메시지:

```
[miraeboy] Upload mylib/1.0.0@sc/dev

Repository: extralib
Package: mylib/1.0.0@sc/dev
Revision: abc123def456
Uploaded by: dev1
```

---

## Git 호스팅 서비스별 토큰 설정

=== "GitHub"

    1. GitHub → Settings → Developer settings → Personal access tokens → Fine-grained tokens
    2. 필요한 권한:
       - **Repository access**: 대상 저장소만 선택
       - **Repository permissions**: Contents → Read and write
    3. 생성된 토큰을 `--git-token`에 사용

    ```bash
    mboy repo update extralib --git-token ghp_xxxxxxxxxxxxxxxxxxxxxxxx
    ```

=== "GitLab"

    1. GitLab → User Settings → Access Tokens
    2. 필요한 권한: `api` 또는 `write_repository`
    3. 생성된 토큰 사용:

    ```bash
    mboy repo update extralib \
      --git-url https://gitlab.com/mycompany/conan-recipes.git \
      --git-token glpat-xxxxxxxxxxxxxxxx
    ```

=== "Gitea"

    1. Gitea → Settings → Applications → Generate New Token
    2. 생성된 토큰 사용:

    ```bash
    mboy repo update extralib \
      --git-url https://gitea.internal/mycompany/conan-recipes.git \
      --git-token your-gitea-token
    ```

=== "자체 호스팅 Git (SSH)"

    SSH 키 방식은 현재 지원되지 않습니다.
    HTTPS 토큰 방식을 사용하세요.

---

## Git Workspace 설정

miraeboy 서버는 Git 저장소를 로컬에 클론하여 사용합니다.
`git_workspace` 경로를 config.yaml에 설정하세요.

```yaml
server:
  git_workspace: "/var/lib/miraeboy/git-workspace"
```

디렉터리가 없으면 자동으로 생성됩니다.

### 디스크 공간 관리

Git 이력이 쌓이면 workspace가 커질 수 있습니다.
정기적으로 확인하고 필요 시 정리합니다.

```bash
# workspace 크기 확인
du -sh /var/lib/miraeboy/git-workspace/

# Git GC 실행 (서버 정지 후)
cd /var/lib/miraeboy/git-workspace/extralib
git gc --prune=now
```

---

## 동기화 실패 처리

Git 동기화 실패 시 패키지 업로드는 유지되고 경고 로그가 기록됩니다.

로그 예시:

```
WARN  git sync failed for extralib: repository not found or authentication failed
WARN  package mylib/1.0.0@sc/dev was uploaded but recipe sync to git failed
```

### 일반적인 실패 원인

| 원인 | 해결 방법 |
|------|-----------|
| 잘못된 토큰 | `mboy repo update` 로 토큰 갱신 |
| 저장소 존재하지 않음 | GitHub/GitLab에서 저장소 생성 후 재시도 |
| 네트워크 오류 | 서버에서 Git 호스팅 서비스 접근 가능 여부 확인 |
| 브랜치 없음 | Git 저장소에 지정된 브랜치가 있는지 확인 |

!!! tip "Git 저장소 초기 설정"
    Git 저장소는 미리 생성되어 있어야 합니다.
    빈 저장소도 괜찮지만, 최소 1개의 커밋(README 등)이 있어야 브랜치가 존재합니다.

    ```bash
    # GitHub CLI로 저장소 생성
    gh repo create mycompany/conan-recipes --private

    # 초기 커밋
    git clone https://github.com/mycompany/conan-recipes.git
    cd conan-recipes
    echo "# Conan Recipes" > README.md
    git add README.md
    git commit -m "Initial commit"
    git push origin main
    ```

---

## 동기화 비활성화

특정 리포지토리의 Git 동기화를 비활성화하려면 git 설정을 제거합니다.

```bash
# REST API로 git 설정 제거
curl -X PATCH http://localhost:9300/api/repos/extralib \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"git": null}'
```
