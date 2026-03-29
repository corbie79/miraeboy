#!/usr/bin/env sh
# miraeboy-agent installer + 빌드 환경 자동 설정
#
# 사용법:
#   curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.sh | sh
#   curl -fsSL .../install-agent.sh | sh -s -- --skip-devtools
#   curl -fsSL .../install-agent.sh | sh -s -- --version v1.2.0
#
# 옵션:
#   --skip-devtools    빌드 도구 설치 건너뜀
#   --version V        에이전트 버전 지정 (기본: latest)
#   --install-dir DIR  설치 경로 (기본: /usr/local/bin)
#   --base-url URL     릴리즈 base URL
#
set -e

REPO="corbie79/miraeboy"
BASE_URL="${MIRAEBOY_BASE_URL:-https://github.com/${REPO}}"
VERSION="${MIRAEBOY_VERSION:-}"
INSTALL_DIR="${MIRAEBOY_INSTALL_DIR:-}"
BINARY="miraeboy-agent"
SKIP_DEVTOOLS=0

# ─── 인자 파싱 ────────────────────────────────────────────────────────────────

while [ $# -gt 0 ]; do
    case "$1" in
        --skip-devtools) SKIP_DEVTOOLS=1;    shift ;;
        --version)       VERSION="$2";       shift 2 ;;
        --install-dir)   INSTALL_DIR="$2";   shift 2 ;;
        --base-url)      BASE_URL="$2";      shift 2 ;;
        -h|--help)
            echo "Usage: install-agent.sh [--skip-devtools] [--version v1.x.x] [--install-dir DIR]"
            exit 0 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ─── 색상 ─────────────────────────────────────────────────────────────────────

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; RESET='\033[0m'
info()    { printf "${CYAN}==>  ${RESET}%s\n" "$*"; }
success() { printf "${GREEN} ✓   ${RESET}%s\n" "$*"; }
warn()    { printf "${YELLOW}WARN ${RESET}%s\n" "$*"; }
step()    { printf "\n${CYAN}━━━ %s ━━━${RESET}\n" "$*"; }

# ─── OS / ARCH 감지 ───────────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux"  ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) echo "unsupported"; exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "unsupported"; exit 1 ;;
    esac
}

detect_pkg_manager() {
    if command -v apt-get >/dev/null 2>&1; then echo "apt"
    elif command -v dnf     >/dev/null 2>&1; then echo "dnf"
    elif command -v yum     >/dev/null 2>&1; then echo "yum"
    elif command -v pacman  >/dev/null 2>&1; then echo "pacman"
    elif command -v apk     >/dev/null 2>&1; then echo "apk"
    elif command -v zypper  >/dev/null 2>&1; then echo "zypper"
    else echo "unknown"
    fi
}

OS=$(detect_os)
ARCH=$(detect_arch)

# ─── 빌드 도구 설치 (Linux) ───────────────────────────────────────────────────

install_devtools_linux() {
    PKG=$(detect_pkg_manager)
    step "빌드 도구 설치 (Linux / $PKG)"

    SUDO=""
    if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    fi

    case "$PKG" in
        apt)
            info "패키지 목록 업데이트..."
            $SUDO apt-get update -qq
            info "빌드 도구 설치..."
            $SUDO apt-get install -y -qq \
                build-essential \
                cmake \
                ninja-build \
                pkg-config \
                git \
                curl \
                wget \
                python3 \
                python3-pip \
                autoconf \
                automake \
                libtool \
                nasm \
                yasm \
                libssl-dev \
                libffi-dev \
                zlib1g-dev \
                libavcodec-dev \
                libavformat-dev \
                libavutil-dev \
                libswscale-dev \
                ffmpeg
            ;;
        dnf)
            info "패키지 목록 업데이트..."
            $SUDO dnf update -y -q
            info "빌드 도구 설치..."
            $SUDO dnf groupinstall -y "Development Tools" -q
            $SUDO dnf install -y -q \
                cmake \
                ninja-build \
                pkg-config \
                git \
                curl \
                wget \
                python3 \
                python3-pip \
                autoconf \
                automake \
                libtool \
                nasm \
                yasm \
                openssl-devel \
                libffi-devel \
                zlib-devel \
                ffmpeg-free \
                ffmpeg-free-devel
            ;;
        yum)
            info "패키지 목록 업데이트..."
            $SUDO yum update -y -q
            info "빌드 도구 설치..."
            $SUDO yum groupinstall -y "Development Tools" -q
            $SUDO yum install -y -q \
                cmake \
                git \
                curl \
                wget \
                python3 \
                autoconf \
                automake \
                libtool \
                nasm \
                yasm \
                openssl-devel \
                libffi-devel \
                zlib-devel
            # ninja: yum에 없는 경우 수동 설치
            if ! command -v ninja >/dev/null 2>&1; then
                info "ninja-build 수동 설치..."
                NINJA_VER="1.12.1"
                curl -fsSL "https://github.com/ninja-build/ninja/releases/download/v${NINJA_VER}/ninja-linux.zip" -o /tmp/ninja.zip
                $SUDO unzip -o /tmp/ninja.zip -d /usr/local/bin
                $SUDO chmod +x /usr/local/bin/ninja
                rm /tmp/ninja.zip
            fi
            ;;
        pacman)
            info "패키지 목록 업데이트..."
            $SUDO pacman -Syu --noconfirm -q
            info "빌드 도구 설치..."
            $SUDO pacman -S --noconfirm -q \
                base-devel \
                cmake \
                ninja \
                pkg-config \
                git \
                curl \
                wget \
                python \
                python-pip \
                autoconf \
                automake \
                libtool \
                nasm \
                yasm \
                openssl \
                ffmpeg
            ;;
        apk)
            info "패키지 목록 업데이트..."
            $SUDO apk update -q
            info "빌드 도구 설치..."
            $SUDO apk add -q \
                build-base \
                cmake \
                ninja \
                pkgconf \
                git \
                curl \
                wget \
                python3 \
                py3-pip \
                autoconf \
                automake \
                libtool \
                nasm \
                yasm \
                openssl-dev \
                ffmpeg-dev
            ;;
        zypper)
            info "패키지 목록 업데이트..."
            $SUDO zypper refresh -q
            info "빌드 도구 설치..."
            $SUDO zypper install -y -q \
                -t pattern devel_basis \
                cmake \
                ninja \
                pkg-config \
                git \
                curl \
                python3 \
                autoconf \
                automake \
                libtool \
                nasm \
                yasm \
                libopenssl-devel \
                ffmpeg-4-libavcodec-devel
            ;;
        *)
            warn "패키지 매니저를 감지할 수 없습니다. 빌드 도구를 수동으로 설치하세요."
            return 0
            ;;
    esac
    success "빌드 도구 설치 완료"
}

# ─── 빌드 도구 설치 (macOS) ───────────────────────────────────────────────────

install_devtools_darwin() {
    step "빌드 도구 설치 (macOS)"

    # Xcode Command Line Tools
    if ! xcode-select -p >/dev/null 2>&1; then
        info "Xcode Command Line Tools 설치..."
        xcode-select --install 2>/dev/null || true
        # 설치 완료 대기
        until xcode-select -p >/dev/null 2>&1; do sleep 5; done
        success "Xcode CLT 설치 완료"
    else
        success "Xcode Command Line Tools 이미 설치됨"
    fi

    # Homebrew
    if ! command -v brew >/dev/null 2>&1; then
        info "Homebrew 설치..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        # Apple Silicon PATH 설정
        if [ -f "/opt/homebrew/bin/brew" ]; then
            eval "$(/opt/homebrew/bin/brew shellenv)"
            # 쉘 프로파일에 추가
            for profile in ~/.zprofile ~/.bash_profile ~/.profile; do
                if [ -f "$profile" ]; then
                    echo 'eval "$(/opt/homebrew/bin/brew shellenv)"' >> "$profile"
                    break
                fi
            done
        fi
        success "Homebrew 설치 완료"
    else
        info "Homebrew 업데이트..."
        brew update --quiet
        success "Homebrew 업데이트 완료"
    fi

    info "빌드 도구 설치..."
    brew install --quiet \
        cmake \
        ninja \
        pkg-config \
        openssl \
        nasm \
        yasm \
        autoconf \
        automake \
        libtool \
        ffmpeg \
        git \
        wget \
        python3 2>/dev/null || true

    success "빌드 도구 설치 완료"
}

# ─── Go 설치 확인 ─────────────────────────────────────────────────────────────

ensure_go() {
    if command -v go >/dev/null 2>&1; then
        GO_VER=$(go version | awk '{print $3}')
        success "Go 이미 설치됨: $GO_VER"
        return 0
    fi

    step "Go 설치"
    GO_VERSION="1.22.4"
    info "Go ${GO_VERSION} 설치..."

    case "${OS}-${ARCH}" in
        linux-amd64)  GO_FILE="go${GO_VERSION}.linux-amd64.tar.gz" ;;
        linux-arm64)  GO_FILE="go${GO_VERSION}.linux-arm64.tar.gz" ;;
        darwin-amd64) GO_FILE="go${GO_VERSION}.darwin-amd64.tar.gz" ;;
        darwin-arm64) GO_FILE="go${GO_VERSION}.darwin-arm64.tar.gz" ;;
        *) warn "Go 자동 설치를 지원하지 않는 플랫폼입니다. 수동으로 설치하세요: https://go.dev/dl/"; return 0 ;;
    esac

    SUDO=""
    if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then SUDO="sudo"; fi

    curl -fsSL "https://go.dev/dl/${GO_FILE}" -o "/tmp/${GO_FILE}"
    $SUDO rm -rf /usr/local/go
    $SUDO tar -C /usr/local -xzf "/tmp/${GO_FILE}"
    rm "/tmp/${GO_FILE}"

    # PATH 설정
    export PATH="$PATH:/usr/local/go/bin"
    for profile in ~/.zshrc ~/.bashrc ~/.profile; do
        if [ -f "$profile" ] && ! grep -q "/usr/local/go/bin" "$profile"; then
            echo 'export PATH="$PATH:/usr/local/go/bin"' >> "$profile"
        fi
    done
    success "Go ${GO_VERSION} 설치 완료"
}

# ─── 에이전트 바이너리 설치 ───────────────────────────────────────────────────

install_agent_binary() {
    step "miraeboy-agent 설치"

    if [ -z "$VERSION" ]; then
        info "최신 버전 조회..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' \
            | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            warn "버전 자동 조회 실패. MIRAEBOY_VERSION 환경변수로 직접 지정하세요."
            return 1
        fi
        info "최신 버전: $VERSION"
    fi

    if [ "$OS" = "windows" ]; then
        ARCHIVE="${BINARY}-${OS}-${ARCH}-${VERSION}.zip"
        DEST_NAME="${BINARY}.exe"
    else
        ARCHIVE="${BINARY}-${OS}-${ARCH}-${VERSION}.tar.gz"
        DEST_NAME="${BINARY}"
    fi

    if [ -z "$INSTALL_DIR" ]; then
        if [ -w "/usr/local/bin" ]; then INSTALL_DIR="/usr/local/bin"
        else INSTALL_DIR="${HOME}/.local/bin"; mkdir -p "$INSTALL_DIR"; fi
    fi

    DOWNLOAD_URL="${BASE_URL}/releases/download/${VERSION}/${ARCHIVE}"
    CHECKSUM_URL="${BASE_URL}/releases/download/${VERSION}/checksums.txt"
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    info "다운로드: $ARCHIVE"
    curl -fsSL --progress-bar -o "${TMP_DIR}/${ARCHIVE}" "$DOWNLOAD_URL"

    if curl -fsSL -o "${TMP_DIR}/checksums.txt" "$CHECKSUM_URL" 2>/dev/null; then
        printf "  체크섬 검증..."
        if (cd "$TMP_DIR" && grep "  ${ARCHIVE}$" checksums.txt | sha256sum -c --status 2>/dev/null); then
            printf " OK\n"
        else
            printf " FAILED\n"
            echo "체크섬 불일치!" >&2; exit 1
        fi
    fi

    info "압축 해제..."
    if [ "$OS" = "windows" ]; then
        unzip -q "${TMP_DIR}/${ARCHIVE}" -d "$TMP_DIR"
    else
        tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"
    fi
    EXTRACTED="${TMP_DIR}/${BINARY}-${OS}-${ARCH}"
    [ "$OS" = "windows" ] && EXTRACTED="${EXTRACTED}.exe"
    chmod +x "$EXTRACTED" 2>/dev/null || true

    DEST="${INSTALL_DIR}/${DEST_NAME}"
    info "설치: $DEST"
    if ! mv "$EXTRACTED" "$DEST" 2>/dev/null; then
        sudo mv "$EXTRACTED" "$DEST" || { echo "설치 실패: $INSTALL_DIR 에 쓰기 권한 없음" >&2; exit 1; }
    fi
    success "miraeboy-agent $VERSION 설치 완료 → $DEST"
}

# ─── PATH 경고 ────────────────────────────────────────────────────────────────

check_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            warn "$INSTALL_DIR 이 PATH에 없습니다."
            echo "  ~/.bashrc 또는 ~/.zshrc에 추가하세요:"
            echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
            ;;
    esac
}

# ─── 메인 ─────────────────────────────────────────────────────────────────────

echo ""
echo "  miraeboy-agent 설치 스크립트"
echo "  OS: ${OS} / ARCH: ${ARCH}"
echo ""

if [ "$SKIP_DEVTOOLS" -eq 0 ]; then
    case "$OS" in
        linux)  install_devtools_linux  ;;
        darwin) install_devtools_darwin ;;
        windows)
            warn "Windows에서는 install-agent.ps1 을 사용하세요."
            warn "PowerShell: irm .../install-agent.ps1 | iex"
            ;;
    esac
    ensure_go
else
    info "--skip-devtools: 빌드 도구 설치 건너뜀"
fi

install_agent_binary
check_path

echo ""
echo "  Quick start:"
echo "    miraeboy-agent --server http://miraeboy.example.com:9300 --agent-key YOUR_KEY"
echo ""
echo "  systemd 서비스로 등록하려면:"
echo "    sudo tee /etc/systemd/system/miraeboy-agent.service <<EOF"
echo "    [Unit]"
echo "    Description=miraeboy build agent"
echo "    After=network.target"
echo ""
echo "    [Service]"
echo "    ExecStart=${INSTALL_DIR}/miraeboy-agent --server http://SERVER:9300 --agent-key KEY"
echo "    Restart=always"
echo ""
echo "    [Install]"
echo "    WantedBy=multi-user.target"
echo "    EOF"
echo "    sudo systemctl enable --now miraeboy-agent"
