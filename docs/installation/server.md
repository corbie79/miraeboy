# 서버 설치

miraeboy 서버(`miraeboy` 바이너리)를 설치하는 방법을 설명합니다.

---

## 시스템 요구사항

| 항목 | 최소 사양 | 권장 사양 |
|------|-----------|-----------|
| OS | Linux (x86_64, arm64), macOS | Linux x86_64 |
| CPU | 2코어 | 4코어 이상 |
| 메모리 | 512MB | 2GB 이상 |
| 디스크 | 1GB (바이너리) | 로컬 스토리지 사용 시 충분한 여유 공간 |
| 포트 | 9300 (기본) | — |

---

## 설치 방법

=== "스크립트 설치 (권장)"

    ### Linux / macOS

    아래 명령어 하나로 최신 버전을 자동으로 다운로드하고 설치합니다.

    ```bash
    curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh
    ```

    설치 후 바이너리 위치를 확인합니다.

    ```bash
    which miraeboy
    miraeboy --version
    ```

    ### Windows (PowerShell)

    PowerShell을 **관리자 권한**으로 실행한 후 입력합니다.

    ```powershell
    irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install.ps1 | iex
    ```

=== "수동 설치"

    ### 바이너리 직접 다운로드

    [GitHub Releases](https://github.com/corbie79/miraeboy/releases) 페이지에서 플랫폼에 맞는 바이너리를 다운로드합니다.

    ```bash
    # Linux x86_64 예시 (버전은 실제 릴리즈 버전으로 변경)
    VERSION=v1.0.0
    curl -fsSL "https://github.com/corbie79/miraeboy/releases/download/${VERSION}/miraeboy-linux-amd64.tar.gz" \
      -o miraeboy.tar.gz
    tar -xzf miraeboy.tar.gz
    sudo mv miraeboy /usr/local/bin/
    sudo chmod +x /usr/local/bin/miraeboy
    ```

    ```bash
    # Linux arm64 예시
    VERSION=v1.0.0
    curl -fsSL "https://github.com/corbie79/miraeboy/releases/download/${VERSION}/miraeboy-linux-arm64.tar.gz" \
      -o miraeboy.tar.gz
    tar -xzf miraeboy.tar.gz
    sudo mv miraeboy /usr/local/bin/
    ```

---

## 초기 설정

### config.yaml 생성

서버를 시작하기 전에 설정 파일을 만들어야 합니다. 아래는 최소 설정 예시입니다.

```yaml
# /etc/miraeboy/config.yaml

server:
  address: ":9300"
  storage_path: "/var/lib/miraeboy/data"
  node_role: "primary"

auth:
  jwt_secret: "반드시-변경하세요-32자-이상의-랜덤-문자열"
  users:
    - username: "admin"
      password: "admin123"
      admin: true

repositories: []
```

!!! warning "보안 주의"
    프로덕션 환경에서는 `jwt_secret`을 반드시 강력한 랜덤 문자열로 변경하세요.
    ```bash
    # 안전한 시크릿 생성 예시
    openssl rand -hex 32
    ```

### 데이터 디렉터리 생성

```bash
sudo mkdir -p /var/lib/miraeboy/data
sudo mkdir -p /etc/miraeboy
sudo chown -R miraeboy:miraeboy /var/lib/miraeboy
```

---

## 서버 실행

### 직접 실행

```bash
miraeboy --config /etc/miraeboy/config.yaml
```

기본 설정 파일 경로 없이 실행하면 현재 디렉터리의 `config.yaml`을 사용합니다.

```bash
# 현재 디렉터리의 config.yaml 사용
miraeboy
```

### 동작 확인

```bash
curl http://localhost:9300/ping
# 응답: {"status":"ok"}
```

---

## systemd 서비스 등록

Linux에서 서버를 시스템 서비스로 등록하면 부팅 시 자동으로 시작됩니다.

### 서비스 계정 생성

```bash
sudo useradd --system --no-create-home --shell /bin/false miraeboy
sudo chown -R miraeboy:miraeboy /var/lib/miraeboy
sudo chown -R miraeboy:miraeboy /etc/miraeboy
```

### systemd 유닛 파일 작성

```bash
sudo tee /etc/systemd/system/miraeboy.service << 'EOF'
[Unit]
Description=miraeboy Package Registry Server
Documentation=https://corbie79.github.io/miraeboy
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=miraeboy
Group=miraeboy
ExecStart=/usr/local/bin/miraeboy --config /etc/miraeboy/config.yaml
Restart=on-failure
RestartSec=5s

# 보안 강화
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/miraeboy

# 환경 변수 (선택 사항)
# EnvironmentFile=/etc/miraeboy/env

[Install]
WantedBy=multi-user.target
EOF
```

### 서비스 활성화 및 시작

```bash
sudo systemctl daemon-reload
sudo systemctl enable miraeboy
sudo systemctl start miraeboy

# 상태 확인
sudo systemctl status miraeboy

# 로그 확인
sudo journalctl -u miraeboy -f
```

---

## 업그레이드

```bash
# 새 버전 설치 스크립트 재실행
curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh

# systemd 서비스 재시작
sudo systemctl restart miraeboy
```

!!! note "데이터 보존"
    업그레이드 시 `storage_path`에 저장된 패키지 데이터는 유지됩니다.
    S3 스토리지를 사용하는 경우 버킷 데이터도 영향을 받지 않습니다.

---

## 다음 단계

- [에이전트 설치](agent.md) — 빌드 워커 설치
- [CLI 설치](cli.md) — mboy 관리 도구 설치
- [config.yaml 레퍼런스](../configuration/config-yaml.md) — 전체 설정 항목 확인
- [빠른 시작](../guide/quickstart.md) — 첫 번째 리포지토리 만들기
