#!/usr/bin/env bash
# miraeboy 설치 / 업그레이드 / 제거 스크립트
# 사용법:
#   sudo ./install.sh           # 설치 또는 업그레이드
#   sudo ./install.sh uninstall # 제거

set -euo pipefail

# ── 설정 ────────────────────────────────────────────────────────────────────────
BINARY_NAME="miraeboy"
INSTALL_BIN="/usr/local/bin/${BINARY_NAME}"
CONFIG_DIR="/etc/miraeboy"
DATA_DIR="/var/lib/miraeboy/data"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
SERVICE_USER="miraeboy"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "${SCRIPT_DIR}")"

# ── 색상 출력 ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()    { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC} $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }
die()     { error "$*"; exit 1; }

# ── 루트 권한 확인 ───────────────────────────────────────────────────────────────
[[ $EUID -eq 0 ]] || die "root 권한이 필요합니다. sudo ./install.sh 로 실행하세요."

# ── 제거 ────────────────────────────────────────────────────────────────────────
uninstall() {
    info "miraeboy 제거 시작..."

    if systemctl is-active --quiet "${BINARY_NAME}" 2>/dev/null; then
        info "서비스 중지 중..."
        systemctl stop "${BINARY_NAME}"
    fi

    if systemctl is-enabled --quiet "${BINARY_NAME}" 2>/dev/null; then
        info "서비스 비활성화 중..."
        systemctl disable "${BINARY_NAME}"
    fi

    [[ -f "${SERVICE_FILE}" ]] && { rm -f "${SERVICE_FILE}"; systemctl daemon-reload; }
    [[ -f "${INSTALL_BIN}" ]] && rm -f "${INSTALL_BIN}"

    warn "설정 및 데이터는 유지됩니다:"
    warn "  설정: ${CONFIG_DIR}"
    warn "  데이터: ${DATA_DIR}"
    warn "  완전 삭제: sudo rm -rf ${CONFIG_DIR} ${DATA_DIR}"

    if id "${SERVICE_USER}" &>/dev/null; then
        warn "서비스 계정(${SERVICE_USER})은 유지됩니다. 삭제: sudo userdel ${SERVICE_USER}"
    fi

    info "제거 완료."
}

# ── 빌드 ────────────────────────────────────────────────────────────────────────
build_binary() {
    info "바이너리 빌드 중..."

    cd "${PROJECT_DIR}"

    # 프론트엔드 빌드
    if [[ -d "web" && -f "web/package.json" ]]; then
        info "프론트엔드 빌드 중 (npm)..."
        cd web
        npm install --silent
        npm run build --silent
        cd "${PROJECT_DIR}"
    fi

    # S3 태그 여부 결정
    BUILD_TAGS=""
    if go list -m github.com/minio/minio-go/v7 &>/dev/null 2>&1; then
        BUILD_TAGS="-tags s3"
        info "S3 지원 포함하여 빌드합니다 (-tags s3)"
    fi

    go build ${BUILD_TAGS} -ldflags="-s -w" -o "${BINARY_NAME}" .
    info "빌드 완료: ${PROJECT_DIR}/${BINARY_NAME}"
}

# ── 설치 ────────────────────────────────────────────────────────────────────────
install() {
    local is_upgrade=false
    [[ -f "${INSTALL_BIN}" ]] && is_upgrade=true

    # ── 바이너리 빌드 ──
    local binary_path="${PROJECT_DIR}/${BINARY_NAME}"
    if [[ ! -f "${binary_path}" ]]; then
        build_binary
    else
        info "기존 빌드 바이너리 사용: ${binary_path}"
        read -r -p "다시 빌드하시겠습니까? [y/N] " rebuild
        [[ "${rebuild,,}" == "y" ]] && build_binary
    fi

    # ── 서비스 계정 ──
    if ! id "${SERVICE_USER}" &>/dev/null; then
        info "서비스 계정 생성: ${SERVICE_USER}"
        useradd --system --no-create-home --shell /usr/sbin/nologin "${SERVICE_USER}"
    fi

    # ── 디렉토리 ──
    info "디렉토리 생성 중..."
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}"
    chown "${SERVICE_USER}:${SERVICE_USER}" "${DATA_DIR}"
    chmod 750 "${DATA_DIR}"
    chown root:${SERVICE_USER} "${CONFIG_DIR}"
    chmod 750 "${CONFIG_DIR}"

    # ── 설정 파일 (첫 설치 시에만 복사) ──
    if [[ ! -f "${CONFIG_DIR}/config.yaml" ]]; then
        info "기본 설정 파일 복사: ${CONFIG_DIR}/config.yaml"
        cp "${PROJECT_DIR}/config.yaml" "${CONFIG_DIR}/config.yaml"
        chown root:${SERVICE_USER} "${CONFIG_DIR}/config.yaml"
        chmod 640 "${CONFIG_DIR}/config.yaml"

        # storage_path 자동 수정
        sed -i "s|storage_path: \"./data\"|storage_path: \"${DATA_DIR}\"|g" \
            "${CONFIG_DIR}/config.yaml"

        warn "⚠️  ${CONFIG_DIR}/config.yaml 을 반드시 편집하세요:"
        warn "   - jwt_secret: 강력한 랜덤 값으로 변경"
        warn "   - auth.users: 비밀번호 변경"
        warn "   - server.address: 필요 시 포트 변경"
    else
        info "기존 설정 파일 유지: ${CONFIG_DIR}/config.yaml"
    fi

    # ── 서비스 업그레이드 시 중지 ──
    if ${is_upgrade} && systemctl is-active --quiet "${BINARY_NAME}" 2>/dev/null; then
        info "기존 서비스 중지 중..."
        systemctl stop "${BINARY_NAME}"
    fi

    # ── 바이너리 설치 ──
    info "바이너리 설치: ${INSTALL_BIN}"
    cp "${binary_path}" "${INSTALL_BIN}"
    chown root:root "${INSTALL_BIN}"
    chmod 755 "${INSTALL_BIN}"

    # ── systemd 서비스 ──
    info "systemd 서비스 설치: ${SERVICE_FILE}"
    cp "${SCRIPT_DIR}/miraeboy.service" "${SERVICE_FILE}"
    # DATA_DIR 경로를 서비스 파일에 반영
    sed -i "s|ReadWritePaths=/var/lib/miraeboy|ReadWritePaths=${DATA_DIR%/data}|g" \
        "${SERVICE_FILE}"
    chmod 644 "${SERVICE_FILE}"

    systemctl daemon-reload
    systemctl enable "${BINARY_NAME}"

    # ── 시작 ──
    if ${is_upgrade}; then
        info "서비스 재시작 중..."
        systemctl start "${BINARY_NAME}"
        info "업그레이드 완료."
    else
        info "서비스 시작 중..."
        systemctl start "${BINARY_NAME}"
        info "설치 완료."
    fi

    echo ""
    info "상태 확인:  sudo systemctl status ${BINARY_NAME}"
    info "로그 보기:  sudo journalctl -u ${BINARY_NAME} -f"
    info "설정 파일:  ${CONFIG_DIR}/config.yaml"
    info "데이터:     ${DATA_DIR}"
    echo ""

    local port
    port=$(grep -E '^\s*address:' "${CONFIG_DIR}/config.yaml" 2>/dev/null \
        | head -1 | sed 's/.*:\([0-9]*\).*/\1/')
    [[ -n "${port}" ]] && info "웹 UI:      http://$(hostname -I | awk '{print $1}'):${port}"
}

# ── 진입점 ────────────────────────────────────────────────────────────────────
case "${1:-install}" in
    uninstall|remove) uninstall ;;
    install|upgrade)  install   ;;
    *) die "사용법: $0 [install|uninstall]" ;;
esac
