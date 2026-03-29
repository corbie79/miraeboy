#!/usr/bin/env sh
# miraeboy / miraeboy-agent universal installer
#
# 사용법:
#   # 서버 설치
#   curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh
#
#   # 에이전트 설치
#   curl -fsSL https://raw.githubusercontent.com/corbie79/miraeboy/main/install.sh | sh -s -- --agent
#
#   # 버전 지정
#   curl -fsSL .../install.sh | sh -s -- --version v1.2.0
#
#   # 설치 경로 지정
#   curl -fsSL .../install.sh | sh -s -- --install-dir ~/.local/bin
#
# 환경변수:
#   MIRAEBOY_VERSION      설치할 버전 (기본값: latest)
#   MIRAEBOY_INSTALL_DIR  설치 경로  (기본값: /usr/local/bin)
#   MIRAEBOY_BASE_URL     릴리즈 base URL (기본값: GitHub)
#
set -e

REPO="corbie79/miraeboy"
BASE_URL="${MIRAEBOY_BASE_URL:-https://github.com/${REPO}}"
VERSION="${MIRAEBOY_VERSION:-}"
INSTALL_DIR="${MIRAEBOY_INSTALL_DIR:-}"
BINARY="miraeboy"   # --agent 플래그로 miraeboy-agent로 변경

# ─── 인자 파싱 ────────────────────────────────────────────────────────────────

while [ $# -gt 0 ]; do
    case "$1" in
        --agent)       BINARY="miraeboy-agent"; shift ;;
        --version)     VERSION="$2";            shift 2 ;;
        --install-dir) INSTALL_DIR="$2";        shift 2 ;;
        --base-url)    BASE_URL="$2";           shift 2 ;;
        -h|--help)
            echo "Usage: install.sh [--agent] [--version v1.x.x] [--install-dir DIR]"
            exit 0 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ─── OS / ARCH 감지 ───────────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux*)            echo "linux"   ;;
        Darwin*)           echo "darwin"  ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) printf "Error: unsupported OS: %s\n" "$(uname -s)" >&2; exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) printf "Error: unsupported arch: %s\n" "$(uname -m)" >&2; exit 1 ;;
    esac
}

OS=$(detect_os)
ARCH=$(detect_arch)

# ─── 최신 버전 조회 ───────────────────────────────────────────────────────────

if [ -z "$VERSION" ]; then
    printf "==> Fetching latest release... "
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "FAILED"
        echo "Error: Could not determine latest version." >&2
        echo "Set MIRAEBOY_VERSION or pass --version vX.Y.Z" >&2
        exit 1
    fi
    echo "$VERSION"
fi

# ─── 다운로드 URL 결정 ────────────────────────────────────────────────────────

if [ "$OS" = "windows" ]; then
    ARCHIVE="${BINARY}-${OS}-${ARCH}-${VERSION}.zip"
else
    ARCHIVE="${BINARY}-${OS}-${ARCH}-${VERSION}.tar.gz"
fi

DOWNLOAD_URL="${BASE_URL}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUM_URL="${BASE_URL}/releases/download/${VERSION}/checksums.txt"

# ─── 설치 경로 결정 ───────────────────────────────────────────────────────────

if [ -z "$INSTALL_DIR" ]; then
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
fi

# ─── 다운로드 ─────────────────────────────────────────────────────────────────

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> Installing ${BINARY} ${VERSION} (${OS}/${ARCH})"
echo "==> Downloading ${ARCHIVE}..."
curl -fsSL --progress-bar -o "${TMP_DIR}/${ARCHIVE}" "$DOWNLOAD_URL"

# ─── 체크섬 검증 ─────────────────────────────────────────────────────────────

if curl -fsSL -o "${TMP_DIR}/checksums.txt" "$CHECKSUM_URL" 2>/dev/null; then
    printf "==> Verifying checksum... "
    if (cd "$TMP_DIR" && grep "  ${ARCHIVE}$" checksums.txt | sha256sum -c --status 2>/dev/null); then
        echo "OK"
    else
        echo "FAILED"
        echo "Error: checksum mismatch! The download may be corrupted." >&2
        exit 1
    fi
fi

# ─── 압축 해제 ───────────────────────────────────────────────────────────────

echo "==> Extracting..."
if [ "$OS" = "windows" ]; then
    unzip -q "${TMP_DIR}/${ARCHIVE}" -d "$TMP_DIR"
    EXTRACTED="${TMP_DIR}/${BINARY}-${OS}-${ARCH}.exe"
    DEST="${INSTALL_DIR}/${BINARY}.exe"
else
    tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"
    EXTRACTED="${TMP_DIR}/${BINARY}-${OS}-${ARCH}"
    DEST="${INSTALL_DIR}/${BINARY}"
fi
chmod +x "$EXTRACTED"

# ─── 설치 ─────────────────────────────────────────────────────────────────────

echo "==> Installing to ${DEST}..."
if ! mv "$EXTRACTED" "$DEST" 2>/dev/null; then
    if command -v sudo >/dev/null 2>&1; then
        sudo mv "$EXTRACTED" "$DEST"
    else
        echo "Error: cannot write to ${INSTALL_DIR}." >&2
        echo "Try: --install-dir ~/.local/bin  or run as root." >&2
        exit 1
    fi
fi

# ─── 완료 ─────────────────────────────────────────────────────────────────────

echo ""
printf "  \033[32m✓\033[0m %s %s installed → %s\n" "$BINARY" "$VERSION" "$DEST"
echo ""

# PATH에 없으면 경고
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        printf "  \033[33mWARNING:\033[0m %s is not in PATH.\n" "$INSTALL_DIR"
        echo "  Add to your shell profile (~/.bashrc, ~/.zshrc …):"
        printf "    export PATH=\"\$PATH:%s\"\n" "$INSTALL_DIR"
        echo ""
        ;;
esac

if [ "$BINARY" = "miraeboy" ]; then
    echo "  Quick start:"
    echo "    miraeboy --help"
    echo "    # config.yaml 편집 후:"
    echo "    miraeboy"
else
    echo "  Quick start:"
    echo "    miraeboy-agent --server http://miraeboy.example.com:9300 --agent-key YOUR_KEY"
fi
