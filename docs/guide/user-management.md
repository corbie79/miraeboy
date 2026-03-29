# 사용자 관리

miraeboy의 사용자 관리 방법을 설명합니다. 로컬 계정 생성/수정/삭제, 권한 설정, 그리고 관련 API를 다룹니다.

---

## 사용자 유형

| 유형 | 설명 |
|------|------|
| **로컬 계정** | config.yaml 또는 API로 생성하는 일반 계정 |
| **OIDC 계정** | Keycloak, Azure AD 등 외부 IdP를 통해 로그인하는 계정 |
| **관리자 (admin)** | 모든 리포지토리와 사용자에 대한 전체 권한 |

!!! note "OIDC 설정"
    OIDC 계정 설정은 [OIDC SSO 가이드](../configuration/oidc-sso.md)를 참고하세요.

---

## mboy CLI로 사용자 관리

### 사용자 목록 조회

```bash
mboy user list
```

출력 예시:

```
USERNAME   관리자   생성일
admin      ✓       2024-01-01
dev1               2024-01-10
ci                 2024-01-10
```

JSON 출력:

```bash
mboy user list --json
```

```json
[
  {"username": "admin", "admin": true, "created_at": "2024-01-01T00:00:00Z"},
  {"username": "dev1", "admin": false, "created_at": "2024-01-10T09:00:00Z"}
]
```

### 사용자 생성

```bash
# 일반 사용자 생성
mboy user create --username dev2 --password "securePass123"

# 관리자 계정 생성
mboy user create --username ops --password "opsPass456" --admin
```

!!! warning "비밀번호 보안"
    비밀번호는 최소 8자 이상을 권장합니다. 스크립트에서 사용할 경우
    환경 변수로 전달하거나, 비밀번호 관리 도구(Vault 등)를 활용하세요.

### 사용자 정보 조회

```bash
mboy user list
# 현재 상세 조회는 목록에서 확인
```

### 사용자 수정

```bash
# 비밀번호 변경
mboy user update dev2 --password "newPassword789"

# 관리자 권한 부여
mboy user update dev2 --admin true

# 관리자 권한 제거
mboy user update dev2 --admin false
```

### 사용자 삭제

```bash
mboy user delete dev2
```

!!! warning "사용자 삭제 시 주의"
    사용자를 삭제해도 해당 사용자가 소유한 리포지토리는 삭제되지 않습니다.
    리포지토리 소유자를 먼저 변경하거나 리포지토리를 삭제한 후 계정을 삭제하세요.

---

## config.yaml로 초기 사용자 설정

서버 시작 시 config.yaml의 `auth.users` 목록에 정의된 사용자가 자동으로 생성됩니다.

```yaml
auth:
  users:
    - username: "admin"
      password: "admin123"
      admin: true
    - username: "dev1"
      password: "dev123"
      admin: false
    - username: "ci"
      password: "ciToken999"
      admin: false
```

!!! tip "CI/CD 계정 관리"
    CI/CD 파이프라인용 계정(`ci`)을 별도로 만들어 `write` 권한만 부여하면
    최소 권한 원칙을 지킬 수 있습니다.

---

## REST API로 사용자 관리

모든 사용자 관련 API는 **관리자 권한**이 필요합니다.

### 인증 토큰 획득

```bash
# 로그인하여 JWT 토큰 획득
TOKEN=$(curl -s -X POST http://localhost:9300/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.token')
```

### 사용자 목록 조회

```http
GET /api/users
Authorization: Bearer <token>
```

```bash
curl http://localhost:9300/api/users \
  -H "Authorization: Bearer $TOKEN"
```

응답 예시:

```json
[
  {
    "username": "admin",
    "admin": true,
    "created_at": "2024-01-01T00:00:00Z"
  },
  {
    "username": "dev1",
    "admin": false,
    "created_at": "2024-01-10T09:00:00Z"
  }
]
```

### 사용자 생성

```http
POST /api/users
Authorization: Bearer <token>
Content-Type: application/json
```

요청 본문:

```json
{
  "username": "newdev",
  "password": "securePassword123",
  "admin": false
}
```

```bash
curl -X POST http://localhost:9300/api/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"newdev","password":"securePassword123","admin":false}'
```

응답:

```json
{
  "username": "newdev",
  "admin": false,
  "created_at": "2024-03-15T10:00:00Z"
}
```

### 사용자 조회

```http
GET /api/users/{username}
Authorization: Bearer <token>
```

```bash
curl http://localhost:9300/api/users/dev1 \
  -H "Authorization: Bearer $TOKEN"
```

### 사용자 수정

```http
PATCH /api/users/{username}
Authorization: Bearer <token>
Content-Type: application/json
```

```bash
# 비밀번호 변경
curl -X PATCH http://localhost:9300/api/users/dev1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"password":"newPassword456"}'

# 관리자 권한 부여
curl -X PATCH http://localhost:9300/api/users/dev1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"admin":true}'
```

### 사용자 삭제

```http
DELETE /api/users/{username}
Authorization: Bearer <token>
```

```bash
curl -X DELETE http://localhost:9300/api/users/dev1 \
  -H "Authorization: Bearer $TOKEN"
```

---

## 리포지토리 멤버 관리

사용자를 특정 리포지토리의 멤버로 추가하여 세분화된 권한을 부여할 수 있습니다.

권한 계층:

| 권한 | 다운로드 | 업로드 | 삭제 | 설정 변경 |
|------|----------|--------|------|-----------|
| `read` | ✓ | | | |
| `write` | ✓ | ✓ | | |
| `delete` | ✓ | ✓ | ✓ | |
| `owner` | ✓ | ✓ | ✓ | ✓ |

```bash
# 리포지토리 멤버 추가
mboy member add extralib dev1 write

# 멤버 목록 조회
mboy member list extralib

# 권한 변경
mboy member update extralib dev1 delete

# 멤버 제거
mboy member remove extralib dev1
```

자세한 내용은 [리포지토리 관리](repository-management.md)를 참고하세요.

---

## 팀 설정 파일 초기화

`mboy init team` 명령으로 팀 구성에 필요한 설정 파일 템플릿을 생성할 수 있습니다.

```bash
mboy init team \
  --dir ./team-config \
  --server http://miraeboy.example.com:9300 \
  --team devteam
```

생성된 파일을 수정하여 팀 멤버와 권한을 일괄 설정하세요.
