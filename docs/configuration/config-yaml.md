# config.yaml 레퍼런스

miraeboy 서버의 전체 설정 파일 레퍼런스입니다.

---

## 전체 설정 예시

```yaml
server:
  address: ":9300"
  storage_path: "./data"
  node_role: "primary"
  artifactory_compat: false
  git_workspace: "./git-workspace"
  s3:
    endpoint: ""
    bucket: "miraeboy"
    access_key_id: ""
    secret_access_key: ""
    use_ssl: false
    region: ""

build:
  agent_key: "secret-key"
  artifacts_dir: "./artifacts"

auth:
  jwt_secret: "change-me-in-production"
  oidc:
    issuer: "https://keycloak.example.com/realms/company"
    client_id: "miraeboy"
    client_secret: "secret"
    redirect_url: "http://miraeboy.example.com/api/auth/oidc/callback"
    groups_claim: "groups"
    admin_groups: ["miraeboy-admin"]
    group_mappings:
      - group: "devteam"
        repository: "extralib"
        permission: "write"
  users:
    - username: "admin"
      password: "admin123"
      admin: true
    - username: "dev1"
      password: "dev123"
      admin: false

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
    git:
      url: "https://github.com/corp/conan-recipes.git"
      branch: "main"
      token: "ghp_xxxx"
```

---

## server 섹션

서버 기본 동작을 설정합니다.

| 필드 | 타입 | 기본값 | 설명 |
|------|------|--------|------|
| `address` | string | `:9300` | 서버가 바인딩할 주소 및 포트 |
| `storage_path` | string | `./data` | 로컬 스토리지 경로 (S3 미사용 시) |
| `node_role` | string | `primary` | 노드 역할: `primary` 또는 `replica` |
| `artifactory_compat` | bool | `false` | Artifactory 호환 URL 패턴 활성화 |
| `git_workspace` | string | `./git-workspace` | Git 레시피 동기화용 로컬 작업 디렉터리 |

### address 예시

```yaml
server:
  address: ":9300"           # 모든 인터페이스, 포트 9300
  address: "127.0.0.1:9300" # localhost만
  address: "0.0.0.0:8080"   # 포트 변경
```

### node_role

| 값 | 설명 |
|----|------|
| `primary` | 읽기/쓰기 모두 가능한 주 노드 |
| `replica` | 읽기 전용 복제 노드 |

```yaml
server:
  node_role: "replica"
```

!!! note "replica 노드"
    replica 노드는 패키지 업로드를 받지 않습니다.
    primary 노드의 스토리지를 공유(S3 사용 권장)하거나 주기적으로 동기화해야 합니다.

---

## server.s3 섹션

S3 호환 스토리지를 사용할 때 설정합니다. 모든 필드가 비어있으면 로컬 스토리지를 사용합니다.

| 필드 | 타입 | 기본값 | 설명 |
|------|------|--------|------|
| `endpoint` | string | — | S3 엔드포인트 URL (AWS S3는 빈 값) |
| `bucket` | string | `miraeboy` | S3 버킷 이름 |
| `access_key_id` | string | — | S3 액세스 키 ID |
| `secret_access_key` | string | — | S3 시크릿 액세스 키 |
| `use_ssl` | bool | `false` | HTTPS 사용 여부 |
| `region` | string | — | AWS 리전 (AWS S3 사용 시) |

자세한 설정은 [S3 스토리지 가이드](s3-storage.md)를 참고하세요.

---

## build 섹션

빌드 시스템 설정입니다.

| 필드 | 타입 | 기본값 | 설명 |
|------|------|--------|------|
| `agent_key` | string | — | 에이전트 인증 키 (비밀 값) |
| `artifacts_dir` | string | `./artifacts` | 빌드 아티팩트 임시 저장 디렉터리 |

```yaml
build:
  agent_key: "use-a-strong-random-key-here"
  artifacts_dir: "/var/lib/miraeboy/artifacts"
```

!!! warning "agent_key 보안"
    `agent_key`는 에이전트와 서버 간 인증에 사용됩니다.
    강력한 랜덤 문자열을 사용하고, 에이전트의 `MIRAEBOY_AGENT_KEY` 환경 변수와 동일하게 설정하세요.
    ```bash
    openssl rand -hex 32
    ```

---

## auth 섹션

인증 관련 설정입니다.

| 필드 | 타입 | 기본값 | 설명 |
|------|------|--------|------|
| `jwt_secret` | string | — | JWT 서명 비밀 키 (필수) |
| `oidc` | object | — | OIDC SSO 설정 (선택) |
| `users` | array | — | 초기 로컬 사용자 목록 (선택) |

### jwt_secret

JWT 토큰 서명에 사용되는 비밀 키입니다. 최소 32자 이상의 랜덤 문자열을 사용하세요.

```yaml
auth:
  jwt_secret: "your-very-long-random-secret-key-at-least-32-chars"
```

```bash
# 안전한 키 생성
openssl rand -hex 32
```

!!! danger "기본값 사용 금지"
    예시에 있는 `"change-me-in-production"` 값을 그대로 사용하지 마세요.
    서버 재시작 시 모든 기존 토큰이 무효화됩니다.

### auth.oidc 섹션

OIDC SSO 설정입니다. 설정하지 않으면 로컬 인증만 사용합니다.

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `issuer` | string | ✓ | OIDC 발급자 URL |
| `client_id` | string | ✓ | OIDC 클라이언트 ID |
| `client_secret` | string | ✓ | OIDC 클라이언트 시크릿 |
| `redirect_url` | string | ✓ | 인증 후 콜백 URL |
| `groups_claim` | string | — | 그룹 정보가 담긴 JWT 클레임 이름 |
| `admin_groups` | array | — | 관리자 권한을 받을 그룹 이름 목록 |
| `group_mappings` | array | — | 그룹 → 리포지토리 권한 매핑 |

```yaml
auth:
  oidc:
    issuer: "https://keycloak.example.com/realms/company"
    client_id: "miraeboy"
    client_secret: "oidc-client-secret"
    redirect_url: "https://miraeboy.example.com/api/auth/oidc/callback"
    groups_claim: "groups"
    admin_groups:
      - "miraeboy-admin"
      - "platform-team"
    group_mappings:
      - group: "devteam"
        repository: "extralib"
        permission: "write"
      - group: "readonly-users"
        repository: "extralib"
        permission: "read"
```

자세한 설정은 [OIDC SSO 가이드](oidc-sso.md)를 참고하세요.

### auth.users 섹션

서버 시작 시 생성되는 초기 사용자 목록입니다.

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `username` | string | ✓ | 사용자명 |
| `password` | string | ✓ | 비밀번호 |
| `admin` | bool | — | 관리자 여부 (기본값: `false`) |

```yaml
auth:
  users:
    - username: "admin"
      password: "strongPassword123!"
      admin: true
    - username: "ci-bot"
      password: "ciPassword456!"
      admin: false
```

!!! tip "초기 사용자 이후 관리"
    `auth.users` 목록은 서버 시작 시 존재하지 않는 사용자만 생성합니다.
    이미 존재하는 사용자의 비밀번호는 업데이트되지 않습니다.
    운영 중 사용자 관리는 `mboy user` 명령이나 REST API를 사용하세요.

---

## repositories 섹션

초기 리포지토리 목록입니다. 서버 시작 시 존재하지 않는 리포지토리를 생성합니다.

| 필드 | 타입 | 필수 | 설명 |
|------|------|------|------|
| `name` | string | ✓ | 리포지토리 이름 |
| `description` | string | — | 설명 |
| `owner` | string | ✓ | 소유자 사용자명 |
| `allowed_namespaces` | array | — | 허용 Conan 네임스페이스 목록 |
| `allowed_channels` | array | — | 허용 Conan 채널 목록 |
| `anonymous_access` | string | — | 익명 접근 권한: `none`, `read`, `write` |
| `members` | array | — | 초기 멤버 목록 |
| `git.url` | string | — | Git 레시피 동기화 URL |
| `git.branch` | string | — | Git 브랜치 (기본값: `main`) |
| `git.token` | string | — | Git 인증 토큰 |

```yaml
repositories:
  - name: "extralib"
    description: "사내 외부 C/C++ 라이브러리"
    owner: "dev1"
    allowed_namespaces: ["sc", "mycompany"]
    allowed_channels: ["dev", "release", "stable"]
    anonymous_access: "none"
    members:
      - username: "ci"
        permission: "write"
      - username: "readonly-user"
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

---

## 환경 변수로 민감 정보 관리

민감한 설정 값은 환경 변수로 주입할 수 있습니다.

```yaml
auth:
  jwt_secret: "${JWT_SECRET}"
  oidc:
    client_secret: "${OIDC_CLIENT_SECRET}"

server:
  s3:
    access_key_id: "${S3_ACCESS_KEY}"
    secret_access_key: "${S3_SECRET_KEY}"
```

systemd 서비스에서 환경 변수 주입:

```ini
# /etc/miraeboy/env
JWT_SECRET=your-actual-secret
OIDC_CLIENT_SECRET=your-oidc-secret
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
```

```ini
# /etc/systemd/system/miraeboy.service
[Service]
EnvironmentFile=/etc/miraeboy/env
```

---

## 최소 설정 (개발용)

```yaml
server:
  address: ":9300"
  storage_path: "./data"

auth:
  jwt_secret: "dev-only-secret-change-in-production"
  users:
    - username: "admin"
      password: "admin123"
      admin: true

repositories: []
```

## 프로덕션 권장 설정

```yaml
server:
  address: ":9300"
  storage_path: "/var/lib/miraeboy/data"
  node_role: "primary"
  artifactory_compat: false
  git_workspace: "/var/lib/miraeboy/git-workspace"
  s3:
    endpoint: ""
    bucket: "miraeboy-packages"
    access_key_id: "${S3_ACCESS_KEY}"
    secret_access_key: "${S3_SECRET_KEY}"
    use_ssl: true
    region: "ap-northeast-2"

build:
  agent_key: "${AGENT_KEY}"
  artifacts_dir: "/var/lib/miraeboy/artifacts"

auth:
  jwt_secret: "${JWT_SECRET}"
  oidc:
    issuer: "https://keycloak.company.internal/realms/main"
    client_id: "miraeboy"
    client_secret: "${OIDC_CLIENT_SECRET}"
    redirect_url: "https://miraeboy.company.internal/api/auth/oidc/callback"
    groups_claim: "groups"
    admin_groups: ["infra-team"]
  users:
    - username: "admin"
      password: "${ADMIN_PASSWORD}"
      admin: true

repositories: []
```
