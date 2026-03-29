# 에이전트 설치

`miraeboy-agent`는 빌드 워커 바이너리로, 서버로부터 빌드 작업을 받아 패키지를 빌드하고 업로드합니다.

---

## 개요

에이전트는 다음과 같은 흐름으로 동작합니다.

```
miraeboy 서버
    │
    │  빌드 트리거 (POST /api/builds)
    ▼
miraeboy-agent
    │
    ├── Git 저장소 클론/업데이트
    ├── 빌드 환경 설정 (Conan / Cargo)
    ├── 패키지 빌드
    └── miraeboy 서버로 업로드
```

에이전트는 **빌드 도구**가 설치된 별도의 머신에서 실행하는 것을 권장합니다.

---

## 시스템 요구사항

| 항목 | 최소 사양 |
|------|-----------|
| OS | Linux (x86_64, arm64), macOS, Windows |
| CPU | 4코어 이상 (빌드 작업용) |
| 메모리 | 4GB 이상 |
| 디스크 | 빌드 아티팩트 저장 공간 (20GB 이상 권장) |

### 선택적 빌드 도구 (devtools)

에이전트가 Conan/Cargo 패키지를 빌드하려면 다음 도구가 필요합니다.

| 도구 | 용도 |
|------|------|
| `conan` (2.x) | Conan 패키지 빌드 |
| `cmake` | C/C++ 빌드 시스템 |
| GCC / Clang | C/C++ 컴파일러 |
| `rustup` / `cargo` | Rust 패키지 빌드 |
| `git` | 소스 체크아웃 |

---

## 설치 방법

=== "스크립트 설치 (devtools 포함, 권장)"

    개발 도구(conan, cmake, rustup 등)를 함께 설치합니다.

    **Linux / macOS:**
    ```bash
    curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.sh | sh
    ```

    **Windows (PowerShell):**
    ```powershell
    irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.ps1 | iex
    ```

=== "스크립트 설치 (devtools 제외)"

    에이전트 바이너리만 설치하고 빌드 도구는 수동으로 관리합니다.

    **Linux / macOS:**
    ```bash
    curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.sh | sh -- --skip-devtools
    ```

    !!! note "빌드 도구를 이미 갖춘 환경"
        CI 서버나 Docker 이미지처럼 빌드 도구가 이미 설치된 환경에서는 `--skip-devtools` 옵션을 사용하세요.

=== "수동 설치"

    ```bash
    VERSION=v1.0.0
    curl -fsSL "https://github.com/corbie79/miraeboy/releases/download/${VERSION}/miraeboy-agent-linux-amd64.tar.gz" \
      -o miraeboy-agent.tar.gz
    tar -xzf miraeboy-agent.tar.gz
    sudo mv miraeboy-agent /usr/local/bin/
    sudo chmod +x /usr/local/bin/miraeboy-agent
    ```

---

## 에이전트 설정

에이전트는 환경 변수 또는 설정 파일로 구성합니다.

### 환경 변수 방식

```bash
export MIRAEBOY_SERVER=http://miraeboy.example.com:9300
export MIRAEBOY_AGENT_KEY=secret-key   # config.yaml의 build.agent_key와 일치해야 함
export MIRAEBOY_ARTIFACTS_DIR=/var/lib/miraeboy-agent/artifacts
```

### 설정 파일 방식

```yaml
# /etc/miraeboy-agent/config.yaml
server_url: "http://miraeboy.example.com:9300"
agent_key: "secret-key"
artifacts_dir: "/var/lib/miraeboy-agent/artifacts"
concurrency: 2   # 동시 빌드 수
```

!!! warning "agent_key 보안"
    `agent_key`는 서버의 `config.yaml`에 설정한 `build.agent_key`와 동일해야 합니다.
    네트워크를 통해 전송되므로 TLS(HTTPS)를 사용하는 환경에서 운영하세요.

---

## 에이전트 실행

```bash
# 환경 변수 사용
MIRAEBOY_SERVER=http://localhost:9300 \
MIRAEBOY_AGENT_KEY=secret-key \
miraeboy-agent

# 설정 파일 사용
miraeboy-agent --config /etc/miraeboy-agent/config.yaml
```

---

## systemd 서비스 등록

### 서비스 계정 및 디렉터리 생성

```bash
sudo useradd --system --no-create-home --shell /bin/false miraeboy-agent
sudo mkdir -p /var/lib/miraeboy-agent/artifacts
sudo mkdir -p /etc/miraeboy-agent
sudo chown -R miraeboy-agent:miraeboy-agent /var/lib/miraeboy-agent
sudo chown -R miraeboy-agent:miraeboy-agent /etc/miraeboy-agent
```

### systemd 유닛 파일

```bash
sudo tee /etc/systemd/system/miraeboy-agent.service << 'EOF'
[Unit]
Description=miraeboy Build Agent
Documentation=https://corbie79.github.io/miraeboy
After=network.target

[Service]
Type=simple
User=miraeboy-agent
Group=miraeboy-agent
EnvironmentFile=/etc/miraeboy-agent/env
ExecStart=/usr/local/bin/miraeboy-agent --config /etc/miraeboy-agent/config.yaml
Restart=on-failure
RestartSec=10s

# 빌드 작업을 위한 디렉터리 접근 허용
ReadWritePaths=/var/lib/miraeboy-agent
ReadWritePaths=/tmp

[Install]
WantedBy=multi-user.target
EOF
```

### 환경 파일 생성

```bash
sudo tee /etc/miraeboy-agent/env << 'EOF'
MIRAEBOY_SERVER=http://miraeboy.example.com:9300
MIRAEBOY_AGENT_KEY=your-secret-agent-key
EOF
sudo chmod 600 /etc/miraeboy-agent/env
sudo chown miraeboy-agent:miraeboy-agent /etc/miraeboy-agent/env
```

### 서비스 시작

```bash
sudo systemctl daemon-reload
sudo systemctl enable miraeboy-agent
sudo systemctl start miraeboy-agent

# 상태 확인
sudo systemctl status miraeboy-agent
sudo journalctl -u miraeboy-agent -f
```

---

## Windows 서비스 등록

Windows에서는 NSSM(Non-Sucking Service Manager)을 사용하거나 PowerShell로 서비스를 등록합니다.

```powershell
# PowerShell (관리자 권한)
New-Service -Name "MiraeboyAgent" `
  -BinaryPathName "C:\miraeboy\miraeboy-agent.exe --config C:\miraeboy\agent-config.yaml" `
  -DisplayName "miraeboy Build Agent" `
  -StartupType Automatic

Start-Service MiraeboyAgent
```

---

## 다중 에이전트 운영

여러 에이전트를 동시에 운영하면 빌드 병렬화가 가능합니다.

```
miraeboy 서버
    │
    ├──► agent-linux   (Linux/amd64 빌드)
    ├──► agent-windows (Windows 빌드)
    └──► agent-macos   (macOS 빌드)
```

각 에이전트는 동일한 `agent_key`를 사용하되, 서로 다른 머신에서 실행합니다.

빌드 트리거 시 `--platforms` 플래그로 특정 플랫폼을 지정할 수 있습니다.

```bash
mboy build trigger --repo mylib --platforms linux/amd64,windows/amd64
```

---

## 다음 단계

- [빌드 시스템 가이드](../guide/build-system.md) — 빌드 트리거 및 관리
- [CLI 설치](cli.md) — mboy 관리 도구 설치
