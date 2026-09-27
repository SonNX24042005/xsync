#!/usr/bin/env bash
# ==============================================================================
# Script cài đặt xsync (Parallel rsync CLI / TUI)
# Cách dùng:
#   curl -fsSL https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.sh | bash
# Hoặc chạy cục bộ:
#   ./install.sh
# ==============================================================================

set -e

REPO="${XSYNC_REPO:-"SonNX24042005/xsync"}"
BINARY_NAME="xsync"
INSTALL_DIR="${INSTALL_DIR:-"$HOME/.local/bin"}"

C_RESET='\033[0m'
C_CYAN='\033[0;36m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[0;33m'
C_RED='\033[0;31m'
C_BOLD='\033[1m'

info() { echo -e "  ${C_CYAN}[INFO]${C_RESET} $*"; }
ok()   { echo -e "  ${C_GREEN}[OK]${C_RESET}   $*"; }
warn() { echo -e "  ${C_YELLOW}[WARN]${C_RESET} $*"; }
err()  { echo -e "  ${C_RED}[ERR]${C_RESET}  $*"; }

echo ""
echo -e "${C_CYAN}=== Cài đặt công cụ xsync ===${C_RESET}"
echo ""

# 1. Phát hiện hệ điều hành và kiến trúc CPU
OS_RAW="$(uname -s)"
ARCH_RAW="$(uname -m)"

case "${OS_RAW}" in
    Linux*)  OS="linux" ;;
    Darwin*) OS="darwin" ;;
    *)       err "Hệ điều hành ${OS_RAW} chưa được hỗ trợ."; exit 1 ;;
esac

case "${ARCH_RAW}" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686)     ARCH="386" ;;
    *)             err "Kiến trúc ${ARCH_RAW} chưa được hỗ trợ."; exit 1 ;;
esac

info "Phát hiện hệ điều hành: ${OS} (${ARCH})"

# 2. Tự động kiểm tra và cài đặt các phụ thuộc hệ thống (rsync, sshpass)
echo ""
info "Kiểm tra các công cụ phụ thuộc hệ thống..."

run_sudo() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v sudo >/dev/null 2>&1; then
        sudo "$@"
    else
        return 1
    fi
}

install_linux_deps() {
    local missing=("$@")
    info "Đang tự động cài đặt phụ thuộc: ${missing[*]}..."

    if command -v apt-get >/dev/null 2>&1; then
        run_sudo apt-get update -y && run_sudo apt-get install -y "${missing[@]}"
    elif command -v dnf >/dev/null 2>&1; then
        run_sudo dnf install -y "${missing[@]}" || {
            run_sudo dnf install -y epel-release && run_sudo dnf install -y "${missing[@]}"
        }
    elif command -v yum >/dev/null 2>&1; then
        run_sudo yum install -y "${missing[@]}" || {
            run_sudo yum install -y epel-release && run_sudo yum install -y "${missing[@]}"
        }
    elif command -v pacman >/dev/null 2>&1; then
        run_sudo pacman -Sy --noconfirm "${missing[@]}"
    elif command -v apk >/dev/null 2>&1; then
        run_sudo apk add --no-cache "${missing[@]}"
    elif command -v zypper >/dev/null 2>&1; then
        run_sudo zypper install -y "${missing[@]}"
    else
        warn "Không xác định được trình quản lý gói tương thích để tự động cài đặt."
        return 1
    fi
}

install_darwin_deps() {
    local missing=("$@")
    if command -v brew >/dev/null 2>&1; then
        info "Đang tự động cài đặt phụ thuộc qua Homebrew: ${missing[*]}..."
        for dep in "${missing[@]}"; do
            if [ "${dep}" = "rsync" ]; then
                brew install rsync || true
            elif [ "${dep}" = "sshpass" ]; then
                brew install esolitos/ipa/sshpass || brew install hudochenkov/sshpass/sshpass || true
            fi
        done
    else
        warn "Chưa cài đặt Homebrew trên macOS. Không thể tự động cài phụ thuộc."
        return 1
    fi
}

MISSING_DEPS=()
if ! command -v rsync >/dev/null 2>&1; then
    MISSING_DEPS+=("rsync")
fi
if ! command -v sshpass >/dev/null 2>&1; then
    MISSING_DEPS+=("sshpass")
fi

if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
    warn "Hệ thống chưa có: ${MISSING_DEPS[*]}"
    DEP_INSTALLED=0
    if [ "${OS}" = "linux" ]; then
        if install_linux_deps "${MISSING_DEPS[@]}"; then
            DEP_INSTALLED=1
        fi
    elif [ "${OS}" = "darwin" ]; then
        if install_darwin_deps "${MISSING_DEPS[@]}"; then
            DEP_INSTALLED=1
        fi
    fi

    # Kiểm tra lại sau khi cài đặt
    RECHECK_FAILED=0
    for dep in "${MISSING_DEPS[@]}"; do
        if ! command -v "${dep}" >/dev/null 2>&1; then
            RECHECK_FAILED=1
            warn "Chưa thể cài đặt tự động '${dep}'. Vui lòng cài đặt thủ công."
        fi
    done
    if [ ${RECHECK_FAILED} -eq 0 ]; then
        ok "Đã tự động cài đặt các phụ thuộc thành công!"
    fi
else
    ok "Tất cả phụ thuộc (rsync, sshpass) đã sẵn sàng!"
fi

# 3. Chuẩn bị thư mục cài đặt
mkdir -p "${INSTALL_DIR}"
TARGET_PATH="${INSTALL_DIR}/${BINARY_NAME}"
TMP_DIR="$(mktemp -d)"

cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

INSTALLED=0

# 4. Chiến lược cài đặt binary
echo ""

# Chiến lược 1: Tải pre-built binary từ GitHub Releases
RELEASE_TAR="https://github.com/${REPO}/releases/latest/download/xsync_${OS}_${ARCH}.tar.gz"
RELEASE_BIN="https://github.com/${REPO}/releases/latest/download/xsync_${OS}_${ARCH}"

info "Kiểm tra bản phát hành trên GitHub Releases..."
if curl -fsSL -I "${RELEASE_TAR}" >/dev/null 2>&1; then
    info "Đang tải bản nhị phân nén: ${RELEASE_TAR}..."
    if curl -fsSL "${RELEASE_TAR}" | tar -xz -C "${TMP_DIR}" 2>/dev/null; then
        if [ -f "${TMP_DIR}/${BINARY_NAME}" ]; then
            rm -f "${TARGET_PATH}"
            mv "${TMP_DIR}/${BINARY_NAME}" "${TARGET_PATH}"
            chmod +x "${TARGET_PATH}"
            ok "Đã cài đặt bản nhị phân từ GitHub Releases thành công!"
            INSTALLED=1
        fi
    fi
elif curl -fsSL -I "${RELEASE_BIN}" >/dev/null 2>&1; then
    info "Đang tải bản nhị phân trực tiếp: ${RELEASE_BIN}..."
    if curl -fsSL "${RELEASE_BIN}" -o "${TARGET_PATH}"; then
        chmod +x "${TARGET_PATH}"
        ok "Đã cài đặt bản nhị phân từ GitHub Releases thành công!"
        INSTALLED=1
    fi
fi

# Chiến lược 2: Sử dụng binary có sẵn trong thư mục dự án cục bộ hoặc tải từ repo main
if [ ${INSTALLED} -eq 0 ]; then
    if [ -f "bin/xsync" ] && [ "${OS}" = "linux" ] && [ "${ARCH}" = "amd64" ]; then
        info "Phát hiện tệp binary có sẵn tại bin/xsync. Đang sao chép..."
        rm -f "${TARGET_PATH}"
        cp bin/xsync "${TARGET_PATH}"
        chmod +x "${TARGET_PATH}"
        ok "Đã cài đặt bản nhị phân có sẵn vào ${TARGET_PATH}!"
        INSTALLED=1
    elif [ "${OS}" = "linux" ] && [ "${ARCH}" = "amd64" ]; then
        RAW_URL="https://raw.githubusercontent.com/${REPO}/main/bin/xsync"
        info "Thử tải bản nhị phân có sẵn từ nhánh main của GitHub..."
        if curl -fsSL "${RAW_URL}" -o "${TARGET_PATH}" 2>/dev/null; then
            chmod +x "${TARGET_PATH}"
            if [ -s "${TARGET_PATH}" ] && head -c 4 "${TARGET_PATH}" | grep -q "ELF"; then
                ok "Đã tải và cài đặt bản nhị phân có sẵn từ GitHub thành công!"
                INSTALLED=1
            else
                rm -f "${TARGET_PATH}"
            fi
        fi
    fi
fi

# Chiến lược 3: Fallback tự biên dịch từ mã nguồn nếu máy có sẵn Go compiler
if [ ${INSTALLED} -eq 0 ]; then
    if command -v go >/dev/null 2>&1; then
        info "Phát hiện Go compiler: $(go version). Bắt đầu biên dịch từ mã nguồn..."
        rm -f "${TARGET_PATH}"
        if [ -f "cmd/xsync/main.go" ]; then
            info "Đang biên dịch từ thư mục mã nguồn hiện tại..."
            go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync
        else
            info "Đang tải mã nguồn từ repository..."
            git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src" >/dev/null 2>&1 || true
            if [ -f "${TMP_DIR}/src/cmd/xsync/main.go" ]; then
                (cd "${TMP_DIR}/src" && go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync)
            fi
        fi

        if [ -f "${TARGET_PATH}" ]; then
            chmod +x "${TARGET_PATH}"
            ok "Đã biên dịch và cài đặt binary thành công!"
            INSTALLED=1
        fi
    fi
fi

# Kiểm tra kết quả cài đặt
if [ ${INSTALLED} -eq 0 ]; then
    err "Không thể tự động tải bản nhị phân hoặc biên dịch."
    echo ""
    echo "  Vui lòng thử một trong các cách sau:"
    echo "    1. Cài đặt Go compiler để tự biên dịch: https://go.dev/dl/"
    echo "    2. Tải bản release nhị phân tương ứng với máy tại: https://github.com/${REPO}/releases"
    echo ""
    exit 1
fi

# 5. Cấu hình biến môi trường PATH
CURRENT_SHELL="$(basename "${SHELL:-bash}")"
RC_FILES=()

case "${CURRENT_SHELL}" in
    bash)
        RC_FILES+=("${HOME}/.bashrc")
        RC_FILES+=("${HOME}/.bash_profile")
        ;;
    zsh)
        RC_FILES+=("${HOME}/.zshrc")
        ;;
    fish)
        RC_FILES+=("${HOME}/.config/fish/config.fish")
        ;;
esac
RC_FILES+=("${HOME}/.profile")

if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    for rc in "${RC_FILES[@]}"; do
        if [ -f "${rc}" ]; then
            if ! grep -q 'export PATH=.*\.local/bin' "${rc}" 2>/dev/null; then
                echo '' >> "${rc}"
                echo '# xsync path' >> "${rc}"
                echo 'export PATH="$HOME/.local/bin:$PATH"' >> "${rc}"
                info "Đã thêm ~/.local/bin vào ${rc}"
                break
            fi
        fi
    done
fi

echo ""
ok "Cài đặt xsync thành công tại: ${TARGET_PATH}"
echo ""
echo -e "${C_CYAN}Cách sử dụng:${C_RESET}"
echo "  Mở một terminal mới (hoặc chạy lệnh dưới) rồi gõ xsync:"
echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
echo "  xsync"
echo ""
