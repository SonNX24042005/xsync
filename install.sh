#!/usr/bin/env bash
# ==============================================================================
# Script cài đặt xsync (Parallel rsync CLI / TUI) từ mã nguồn GitHub
# Cách dùng:
#   curl -fsSL https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.sh | bash
# ==============================================================================

set -e

REPO="${XSYNC_REPO:-SonNX24042005/xsync}"
BINARY_NAME="xsync"
INSTALL_DIR="${HOME}/.local/bin"

C_RESET='\033[0m'
C_CYAN='\033[0;36m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[0;33m'
C_RED='\033[0;31m'

info() { echo -e "  ${C_CYAN}[INFO]${C_RESET} $1"; }
ok()   { echo -e "  ${C_GREEN}[OK]${C_RESET}   $1"; }
warn() { echo -e "  ${C_YELLOW}[WARN]${C_RESET} $1"; }
err()  { echo -e "  ${C_RED}[ERR]${C_RESET}  $1"; }

echo ""
echo -e "${C_CYAN}=== Cài đặt công cụ xsync ===${C_RESET}"
echo ""

# 1. Kiểm tra Go compiler
if ! command -v go >/dev/null 2>&1; then
    err "Không tìm thấy Go compiler trên hệ thống."
    echo ""
    echo "  Vui lòng cài đặt Go trước khi tiếp tục:"
    echo "    - Ubuntu/Debian: sudo apt update && sudo apt install -y golang-go"
    echo "    - macOS:         brew install go"
    echo "    - Hoặc tải từ:   https://go.dev/dl/"
    echo ""
    exit 1
fi

info "Phát hiện Go compiler: $(go version)"

# 2. Chuẩn bị thư mục cài đặt
mkdir -p "${INSTALL_DIR}"
TARGET_PATH="${INSTALL_DIR}/${BINARY_NAME}"
TMP_DIR="$(mktemp -d)"

cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

# 3. Biên dịch binary từ mã nguồn
rm -f "${TARGET_PATH}"

if [ -f "cmd/xsync/main.go" ]; then
    info "Đang biên dịch từ thư mục mã nguồn hiện tại..."
    go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync
else
    info "Đang tải mã nguồn mới nhất từ GitHub..."
    git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src" >/dev/null 2>&1
    if [ -f "${TMP_DIR}/src/cmd/xsync/main.go" ]; then
        info "Đang biên dịch binary..."
        (cd "${TMP_DIR}/src" && go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync)
    else
        err "Không thể tải mã nguồn từ repository ${REPO}."
        exit 1
    fi
fi

chmod +x "${TARGET_PATH}"
ok "Đã biên dịch và cài đặt binary thành công!"

# 4. Kiểm tra các công cụ phụ thuộc (rsync, sshpass)
echo ""
info "Kiểm tra các công cụ phụ thuộc..."
MISSING_DEP=0

if ! command -v rsync >/dev/null 2>&1; then
    warn "Chưa tìm thấy 'rsync'. Cài đặt: sudo apt install rsync hoặc brew install rsync"
    MISSING_DEP=1
fi

if ! command -v sshpass >/dev/null 2>&1; then
    warn "Chưa tìm thấy 'sshpass'. Cài đặt: sudo apt install sshpass hoặc brew install esolitos/ipa/sshpass"
    MISSING_DEP=1
fi

if [ ${MISSING_DEP} -eq 0 ]; then
    ok "Tất cả phụ thuộc (rsync, sshpass) đã sẵn sàng!"
fi

# 5. Cấu hình biến môi trường PATH
echo ""
CURRENT_SHELL="$(basename "${SHELL:-bash}")"
RC_FILE=""

case "${CURRENT_SHELL}" in
    zsh)
        RC_FILE="${HOME}/.zshrc"
        ;;
    bash)
        if [ -f "${HOME}/.bashrc" ]; then
            RC_FILE="${HOME}/.bashrc"
        elif [ -f "${HOME}/.bash_profile" ]; then
            RC_FILE="${HOME}/.bash_profile"
        fi
        ;;
    *)
        RC_FILE="${HOME}/.profile"
        ;;
esac

if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    if [ -n "${RC_FILE}" ] && [ -f "${RC_FILE}" ]; then
        if ! grep -q 'export PATH=.*\.local/bin' "${RC_FILE}"; then
            echo '' >> "${RC_FILE}"
            echo '# xsync path' >> "${RC_FILE}"
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "${RC_FILE}"
            info "Đã thêm ~/.local/bin vào ${RC_FILE}"
        fi
    fi
fi

ok "Cài đặt xsync thành công tại: ${TARGET_PATH}"
echo ""
echo -e "${C_CYAN}Cách sử dụng:${C_RESET}"
echo "  Mở một terminal mới (hoặc chạy lệnh dưới) rồi gõ xsync:"
echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
echo "  xsync"
echo ""
