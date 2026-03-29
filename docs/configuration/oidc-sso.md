# OIDC SSO 설정

miraeboy는 OpenID Connect(OIDC) 기반 싱글 사인온(SSO)을 지원합니다.
Keycloak, Azure Active Directory, Google Workspace, Okta 등 표준 OIDC 프로바이더와 연동 가능합니다.

---

## 개요

OIDC 로그인 흐름:

```
사용자 브라우저
    │
    │  GET /api/auth/oidc/login
    ▼
miraeboy 서버
    │
    │  redirect to IdP
    ▼
Keycloak / Azure AD / ...
    │
    │  인증 완료 → callback URL로 리디렉트
    ▼
miraeboy 서버 (GET /api/auth/oidc/callback)
    │
    │  JWT 토큰 발급
    ▼
사용자 (JWT 토큰으로 API 사용)
```

---

## config.yaml OIDC 설정

```yaml
auth:
  jwt_secret: "your-jwt-secret"
  oidc:
    issuer: "https://keycloak.example.com/realms/company"
    client_id: "miraeboy"
    client_secret: "your-client-secret"
    redirect_url: "https://miraeboy.example.com/api/auth/oidc/callback"
    groups_claim: "groups"
    admin_groups:
      - "miraeboy-admin"
    group_mappings:
      - group: "devteam"
        repository: "extralib"
        permission: "write"
      - group: "readonly"
        repository: "extralib"
        permission: "read"
```

### 설정 항목 설명

| 항목 | 필수 | 설명 |
|------|------|------|
| `issuer` | ✓ | OIDC 발급자(Issuer) URL. IdP의 `/.well-known/openid-configuration` 기준 |
| `client_id` | ✓ | IdP에 등록한 클라이언트 ID |
| `client_secret` | ✓ | IdP에 등록한 클라이언트 시크릿 |
| `redirect_url` | ✓ | 인증 후 콜백 URL. miraeboy 외부 접근 주소와 일치해야 함 |
| `groups_claim` | — | 그룹 정보가 담긴 JWT 클레임 이름 (기본: `groups`) |
| `admin_groups` | — | 관리자 권한을 부여할 그룹 이름 목록 |
| `group_mappings` | — | 그룹 → 리포지토리 권한 자동 매핑 |

---

## Keycloak 설정

### Keycloak에서 클라이언트 생성

1. Keycloak Admin Console → 해당 Realm 선택 → Clients
2. "Create client" 클릭
3. 설정:
   - **Client ID**: `miraeboy`
   - **Client type**: OpenID Connect
   - **Client authentication**: ON (Confidential client)
4. 저장 후 Settings 탭:
   - **Root URL**: `https://miraeboy.example.com`
   - **Valid redirect URIs**: `https://miraeboy.example.com/api/auth/oidc/callback`
   - **Web origins**: `https://miraeboy.example.com`
5. Credentials 탭에서 Client secret 복사

### Keycloak 그룹 클레임 설정

JWT 토큰에 그룹 정보를 포함하려면 Client Scope를 설정해야 합니다.

1. Clients → miraeboy → Client scopes → Add client scope
2. "groups" scope 추가 (없으면 Client Scopes 메뉴에서 새로 생성)

새 Client Scope 생성:
1. Client Scopes → Create client scope
2. **Name**: `groups`
3. **Type**: Default
4. Mappers 탭 → Configure a new mapper → Group Membership
   - **Name**: `groups`
   - **Token Claim Name**: `groups`
   - **Full group path**: OFF (그룹 이름만 포함)
   - **Add to ID token**: ON
   - **Add to access token**: ON

### miraeboy config.yaml (Keycloak)

```yaml
auth:
  oidc:
    issuer: "https://keycloak.example.com/realms/company"
    client_id: "miraeboy"
    client_secret: "copied-from-keycloak-credentials-tab"
    redirect_url: "https://miraeboy.example.com/api/auth/oidc/callback"
    groups_claim: "groups"
    admin_groups:
      - "miraeboy-admin"
    group_mappings:
      - group: "backend-team"
        repository: "extralib"
        permission: "write"
```

---

## Azure Active Directory (Microsoft Entra ID) 설정

### Azure AD 앱 등록

1. Azure Portal → Microsoft Entra ID → App registrations → New registration
2. 설정:
   - **Name**: miraeboy
   - **Redirect URI**: `https://miraeboy.example.com/api/auth/oidc/callback`
3. 등록 후 Overview에서 **Application (client) ID** 확인

### 클라이언트 시크릿 생성

1. Certificates & secrets → New client secret
2. 생성된 값 복사 (이후 다시 볼 수 없음)

### 그룹 클레임 추가

1. Token configuration → Add groups claim
2. **Group types**: Security groups
3. **ID**, **Access** 토큰에 포함 선택

### miraeboy config.yaml (Azure AD)

```yaml
auth:
  oidc:
    issuer: "https://login.microsoftonline.com/{TENANT_ID}/v2.0"
    client_id: "{APPLICATION_CLIENT_ID}"
    client_secret: "{CLIENT_SECRET_VALUE}"
    redirect_url: "https://miraeboy.example.com/api/auth/oidc/callback"
    groups_claim: "groups"    # Azure AD는 그룹 OID를 반환
    admin_groups:
      - "00000000-0000-0000-0000-000000000000"   # 그룹 OID 사용
```

!!! note "Azure AD 그룹 클레임"
    Azure AD는 그룹 이름 대신 그룹 OID(오브젝트 ID)를 클레임으로 반환합니다.
    `admin_groups`와 `group_mappings`에 그룹 이름 대신 OID를 사용하세요.

---

## Google Workspace 설정

### Google Cloud Console에서 OAuth 2.0 클라이언트 생성

1. Google Cloud Console → APIs & Services → Credentials
2. Create Credentials → OAuth client ID
3. **Application type**: Web application
4. **Authorized redirect URIs**: `https://miraeboy.example.com/api/auth/oidc/callback`
5. Client ID와 Client secret 복사

### miraeboy config.yaml (Google)

```yaml
auth:
  oidc:
    issuer: "https://accounts.google.com"
    client_id: "xxxx.apps.googleusercontent.com"
    client_secret: "GOCSPX-xxxxxxxxxxxxxxxx"
    redirect_url: "https://miraeboy.example.com/api/auth/oidc/callback"
    groups_claim: ""   # Google은 그룹 클레임을 직접 지원하지 않음
```

!!! note "Google 그룹 클레임 제한"
    Google Workspace는 기본적으로 그룹 클레임을 제공하지 않습니다.
    그룹 기반 권한 관리가 필요하다면 Keycloak을 거치는 Federation을 사용하거나,
    수동으로 사용자에게 권한을 부여하세요.

---

## group_mappings 활용

OIDC 그룹에 따라 리포지토리 접근 권한을 자동으로 부여할 수 있습니다.

```yaml
auth:
  oidc:
    group_mappings:
      # backend 팀은 extralib에 write 권한
      - group: "backend-team"
        repository: "extralib"
        permission: "write"

      # frontend 팀은 extralib에 read 권한
      - group: "frontend-team"
        repository: "extralib"
        permission: "read"

      # devops 팀은 모든 리포지토리에 delete 권한
      - group: "devops-team"
        repository: "extralib"
        permission: "delete"
      - group: "devops-team"
        repository: "rustlib"
        permission: "delete"
```

**동작 방식:**
- 사용자가 OIDC로 로그인할 때 JWT의 그룹 클레임을 확인
- `group_mappings`에 매칭되는 항목이 있으면 해당 리포지토리에 자동 멤버 등록
- 그룹에서 제거되면 다음 로그인 시 권한도 자동으로 제거됨

---

## 로그인 URL

OIDC 로그인 URL을 브라우저에서 직접 접근하거나 프론트엔드에서 링크로 제공합니다.

```
GET https://miraeboy.example.com/api/auth/oidc/login
```

콜백 URL (redirect_url에 설정한 값):

```
GET https://miraeboy.example.com/api/auth/oidc/callback
```

---

## 로컬 계정과 OIDC 공존

OIDC를 활성화해도 로컬 계정(`/api/auth/login`)은 계속 동작합니다.
OIDC 계정과 같은 사용자명의 로컬 계정이 있으면 충돌이 발생할 수 있으므로 주의하세요.

!!! tip "관리자 계정은 로컬 계정 유지"
    OIDC 설정 오류 시 접근이 불가해질 수 있으므로,
    관리자 계정 하나는 항상 로컬 계정으로 유지하는 것을 권장합니다.
