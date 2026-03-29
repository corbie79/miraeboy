# CLI (mboy) 설치

`mboy`는 miraeboy 서버를 관리하는 CLI 도구입니다. 사용자, 리포지토리, 빌드, 패키지를 터미널에서 편리하게 관리할 수 있습니다.

---

## 설치

=== "Linux / macOS"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.sh | sh
    ```

    설치 확인:
    ```bash
    mboy version
    ```

=== "Windows (PowerShell)"

    ```powershell
    irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install-cli.ps1 | iex
    ```

=== "수동 설치"

    ```bash
    # Linux x86_64
    VERSION=v1.0.0
    curl -fsSL "https://github.com/corbie79/miraeboy/releases/download/${VERSION}/mboy-linux-amd64.tar.gz" \
      -o mboy.tar.gz
    tar -xzf mboy.tar.gz
    sudo mv mboy /usr/local/bin/
    sudo chmod +x /usr/local/bin/mboy

    # macOS (Apple Silicon)
    curl -fsSL "https://github.com/corbie79/miraeboy/releases/download/${VERSION}/mboy-darwin-arm64.tar.gz" \
      -o mboy.tar.gz
    tar -xzf mboy.tar.gz
    sudo mv mboy /usr/local/bin/
    ```

---

## 초기 설정

설치 후 서버에 로그인하면 인증 토큰이 저장되어 이후 명령에 자동으로 사용됩니다.

```bash
mboy login --server http://miraeboy.example.com:9300 \
           --user admin \
           --password admin123
```

로그인에 성공하면 토큰이 `~/.mboy/config.json`에 저장됩니다.

```bash
# 현재 연결 상태 확인
mboy status
```

---

## 글로벌 플래그

모든 명령에서 사용할 수 있는 플래그입니다.

| 플래그 | 설명 | 기본값 |
|--------|------|--------|
| `--server URL` | 서버 주소 | `http://localhost:9300` |
| `--token TOKEN` | API 토큰 (로그인 대신 사용 가능) | — |
| `--json` | 출력을 JSON 형식으로 변환 | `false` |
| `--version` | 버전 출력 | — |

### 환경 변수로 기본값 설정

자주 사용하는 서버 주소와 토큰을 환경 변수로 설정하면 편리합니다.

```bash
export MBOY_SERVER=http://miraeboy.example.com:9300
export MBOY_TOKEN=your-jwt-token
```

---

## 셸 자동 완성 설정

`mboy`는 bash, zsh, fish 셸의 자동 완성을 지원합니다.

=== "bash"

    ```bash
    # 자동 완성 스크립트 생성 및 적용
    mboy completion bash > /etc/bash_completion.d/mboy

    # 또는 사용자 홈에 저장
    mboy completion bash > ~/.bash_completion_mboy
    echo 'source ~/.bash_completion_mboy' >> ~/.bashrc
    source ~/.bashrc
    ```

=== "zsh"

    ```zsh
    # Oh My Zsh 사용 시
    mboy completion zsh > "${fpath[1]}/_mboy"

    # 일반 zsh
    mkdir -p ~/.zsh/completions
    mboy completion zsh > ~/.zsh/completions/_mboy
    echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
    echo 'autoload -Uz compinit && compinit' >> ~/.zshrc
    source ~/.zshrc
    ```

=== "fish"

    ```fish
    mboy completion fish > ~/.config/fish/completions/mboy.fish
    ```

자동 완성 설정 후 `mboy <Tab>`을 누르면 사용 가능한 명령어가 표시됩니다.

```
$ mboy <Tab>
build       cargo       completion  init        login       member
package     repo        status      user        version
```

---

## 자주 쓰는 명령 요약

```bash
# 로그인
mboy login --server http://server:9300 --user admin --password admin123

# 상태 확인
mboy status

# 리포지토리 목록
mboy repo list

# 사용자 목록 (admin 전용)
mboy user list

# 빌드 목록
mboy build list

# JSON 출력 (스크립트 연동)
mboy repo list --json | jq '.[].name'
```

---

## 다음 단계

- [빠른 시작 가이드](../guide/quickstart.md) — 실제 사용 예시
- [CLI 전체 레퍼런스](../cli/reference.md) — 모든 명령어 상세 설명
