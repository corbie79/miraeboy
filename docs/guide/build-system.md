# 빌드 시스템

miraeboy의 서버-에이전트 빌드 시스템을 설명합니다.
서버가 빌드 작업을 생성하고, 에이전트가 실제 빌드를 수행합니다.

---

## 아키텍처

```
개발자/CI  ──► mboy build trigger
                     │
                     ▼
              miraeboy 서버
              (빌드 큐 관리)
                     │
          ┌──────────┼──────────┐
          ▼          ▼          ▼
      agent-linux  agent-win  agent-macos
      (빌드 실행)  (빌드 실행) (빌드 실행)
          │
          │ 빌드 성공 시
          ▼
      패키지 업로드 → miraeboy 레지스트리
```

---

## 사전 준비

### 서버 설정

`config.yaml`에 빌드 관련 설정이 필요합니다.

```yaml
build:
  agent_key: "your-secret-agent-key"   # 에이전트 인증 키
  artifacts_dir: "./artifacts"          # 빌드 아티팩트 임시 저장 디렉터리
```

### 에이전트 설치

에이전트가 설치되어 있어야 합니다. 상세 설치 방법은 [에이전트 설치 가이드](../installation/agent.md)를 참고하세요.

```bash
# 빠른 설치 (devtools 포함)
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.sh | sh
```

에이전트가 서버에 연결되도록 설정합니다.

```bash
export MIRAEBOY_SERVER=http://miraeboy.example.com:9300
export MIRAEBOY_AGENT_KEY=your-secret-agent-key
miraeboy-agent &
```

---

## 빌드 트리거

### 기본 빌드 트리거

```bash
mboy build trigger --repo extralib
```

### Git 소스에서 빌드

```bash
mboy build trigger \
  --repo extralib \
  --git-url https://github.com/mycompany/mylib.git \
  --ref main
```

### 특정 브랜치/태그/커밋에서 빌드

```bash
# 브랜치
mboy build trigger --repo extralib --git-url https://... --ref feature/new-api

# 태그
mboy build trigger --repo extralib --git-url https://... --ref v1.2.0

# 커밋 해시
mboy build trigger --repo extralib --git-url https://... --ref abc123def
```

### 플랫폼 지정

```bash
# 특정 플랫폼 지정
mboy build trigger \
  --repo extralib \
  --git-url https://github.com/mycompany/mylib.git \
  --ref main \
  --platforms linux/amd64,windows/amd64

# 지원 플랫폼 형식
#   linux/amd64
#   linux/arm64
#   windows/amd64
#   darwin/amd64
#   darwin/arm64
```

---

## 빌드 목록 조회

```bash
mboy build list
```

출력 예시:

```
ID    리포지토리  상태      시작 시간            플랫폼
42    extralib   success   2024-03-15 10:00:00  linux/amd64
41    extralib   running   2024-03-15 09:55:00  windows/amd64
40    rustlib    failed    2024-03-14 15:30:00  linux/amd64
39    extralib   pending   2024-03-14 14:00:00  linux/arm64
```

### 빌드 상태 값

| 상태 | 설명 |
|------|------|
| `pending` | 에이전트 배정 대기 중 |
| `running` | 빌드 진행 중 |
| `success` | 빌드 및 업로드 성공 |
| `failed` | 빌드 실패 |

---

## 빌드 상세 조회

```bash
mboy build get 42
```

출력 예시:

```
빌드 ID:     42
리포지토리:  extralib
상태:        success
플랫폼:      linux/amd64
Git URL:     https://github.com/mycompany/mylib.git
Git 참조:    v1.2.0
시작:        2024-03-15 10:00:00
완료:        2024-03-15 10:05:30
소요 시간:   5분 30초

빌드 로그:
  [INFO] Cloning repository...
  [INFO] Running: conan create . --user sc --channel release
  ...
  [INFO] Upload completed: mylib/1.2.0@sc/release
```

---

## REST API

### 빌드 목록 조회

```http
GET /api/builds
Authorization: Bearer <token>
```

```bash
curl http://localhost:9300/api/builds \
  -H "Authorization: Bearer $TOKEN"
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

```json
{
  "repository": "extralib",
  "git_url": "https://github.com/mycompany/mylib.git",
  "git_ref": "main",
  "platforms": ["linux/amd64", "windows/amd64"]
}
```

```bash
curl -X POST http://localhost:9300/api/builds \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "repository": "extralib",
    "git_url": "https://github.com/mycompany/mylib.git",
    "git_ref": "main",
    "platforms": ["linux/amd64"]
  }'
```

응답:

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

```bash
curl http://localhost:9300/api/builds/43 \
  -H "Authorization: Bearer $TOKEN"
```

---

## CI/CD 파이프라인 연동

### GitHub Actions

```yaml
# .github/workflows/build.yml
name: Build and Publish

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install mboy
        run: |
          curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.sh | sh

      - name: Login to miraeboy
        run: |
          mboy login \
            --server ${{ secrets.MIRAEBOY_SERVER }} \
            --user ${{ secrets.MIRAEBOY_USER }} \
            --password ${{ secrets.MIRAEBOY_PASSWORD }}

      - name: Trigger Build
        run: |
          BUILD_ID=$(mboy build trigger \
            --repo extralib \
            --git-url ${{ github.server_url }}/${{ github.repository }} \
            --ref ${{ github.ref_name }} \
            --json | jq -r '.id')

          echo "Build ID: $BUILD_ID"

          # 빌드 완료 대기 (폴링)
          for i in $(seq 1 60); do
            STATUS=$(mboy build get $BUILD_ID --json | jq -r '.status')
            echo "상태: $STATUS"
            if [ "$STATUS" = "success" ]; then
              echo "빌드 성공!"
              exit 0
            elif [ "$STATUS" = "failed" ]; then
              echo "빌드 실패!"
              exit 1
            fi
            sleep 10
          done
          echo "타임아웃"
          exit 1
```

### GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - build

build_and_publish:
  stage: build
  script:
    - curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.sh | sh
    - mboy login --server "$MIRAEBOY_SERVER" --user "$MIRAEBOY_USER" --password "$MIRAEBOY_PASSWORD"
    - |
      BUILD_ID=$(mboy build trigger \
        --repo extralib \
        --git-url "$CI_REPOSITORY_URL" \
        --ref "$CI_COMMIT_TAG" \
        --json | jq -r '.id')
      echo "Build ID: $BUILD_ID"
  only:
    - tags
```

---

## 다중 에이전트 운영

다양한 플랫폼을 지원하려면 각 플랫폼별 에이전트를 별도 머신에서 운영합니다.

```
miraeboy 서버
    │
    ├──► Linux 에이전트  (Ubuntu 22.04 LTS)
    │     GCC 12, Clang 15, CMake 3.27
    │     Rust 1.75
    │
    ├──► Windows 에이전트  (Windows Server 2022)
    │     MSVC 2022, CMake 3.27
    │     Rust 1.75
    │
    └──► macOS 에이전트  (macOS 13 Ventura)
          Clang 15, CMake 3.27
          Rust 1.75
```

모든 에이전트는 같은 `agent_key`를 사용합니다.

---

## 아티팩트 관리

빌드 아티팩트는 `build.artifacts_dir`에 임시 저장되고, 업로드 완료 후 정리됩니다.

```yaml
# config.yaml
build:
  artifacts_dir: "/var/lib/miraeboy/artifacts"
```

디스크 사용량을 모니터링하고, 필요 시 주기적으로 정리합니다.

```bash
# 아티팩트 디렉터리 크기 확인
du -sh /var/lib/miraeboy/artifacts
```
