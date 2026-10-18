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

# 0. Lựa chọn ngôn ngữ hiển thị (Language selection)
CHOSEN_LANG="${XSYNC_LANG:-""}"

while [ $# -gt 0 ]; do
    case "$1" in
        --lang)
            CHOSEN_LANG="$2"
            shift 2
            ;;
        --lang=*)
            CHOSEN_LANG="${1#*=}"
            shift
            ;;
        *)
            shift
            ;;
    esac
done

if [ -z "${CHOSEN_LANG}" ]; then
    echo ""
    echo "  Select language / Chọn ngôn ngữ:"
    echo "    1) English (en)"
    echo "    2) Tiếng Việt (vi) [Default / Mặc định]"
    echo ""

    LANG_INPUT=""
    if [ -t 0 ]; then
        read -rp "  Enter choice (1-2) [Default: 2]: " LANG_INPUT
    elif [ -r /dev/tty ]; then
        read -rp "  Enter choice (1-2) [Default: 2]: " LANG_INPUT < /dev/tty
    else
        LANG_INPUT="2"
    fi

    case "${LANG_INPUT}" in
        1|en|EN|english|English)
            CHOSEN_LANG="en"
            ;;
        *)
            CHOSEN_LANG="vi"
            ;;
    esac
fi

# Lưu tùy chọn ngôn ngữ vào ~/.config/xsync/config.json
CONFIG_DIR="${HOME}/.config/xsync"
mkdir -p "${CONFIG_DIR}"
chmod 700 "${CONFIG_DIR}"
umask 077
cat << EOF > "${CONFIG_DIR}/config.json"
{"language":"${CHOSEN_LANG}"}
EOF

if [ "${CHOSEN_LANG}" = "en" ]; then
    MSG_HEADER="=== Installing xsync ==="
    MSG_LANG_SAVED="Default language saved: English (en) in ~/.config/xsync/config.json"
    MSG_CHECK_DEPS="Checking system dependencies..."
    MSG_ALL_DEPS_READY="All dependencies (rsync, sshpass) are ready!"
    MSG_CHECK_RELEASES="Checking releases on GitHub Releases..."
    MSG_RELEASE_INSTALLED="Installed binary from GitHub Releases successfully!"
    MSG_LOCAL_BIN_FOUND="Detected existing binary at bin/xsync. Copying..."
    MSG_LOCAL_BIN_INSTALLED="Installed existing binary to"
    MSG_BUILD_SUCCESS="Compiled and installed binary successfully!"
    MSG_INSTALL_SUCCESS="xsync installed successfully at:"
    MSG_USAGE_TITLE="How to use:"
    MSG_USAGE_HINT="Open a new terminal (or run below) and type xsync:"
else
    MSG_HEADER="=== Cài đặt công cụ xsync ==="
    MSG_LANG_SAVED="Đã lưu cấu hình ngôn ngữ: Tiếng Việt (vi) tại ~/.config/xsync/config.json"
    MSG_CHECK_DEPS="Kiểm tra các công cụ phụ thuộc hệ thống..."
    MSG_ALL_DEPS_READY="Tất cả phụ thuộc (rsync, sshpass) đã sẵn sàng!"
    MSG_CHECK_RELEASES="Kiểm tra bản phát hành trên GitHub Releases..."
    MSG_RELEASE_INSTALLED="Đã cài đặt bản nhị phân từ GitHub Releases thành công!"
    MSG_LOCAL_BIN_FOUND="Phát hiện tệp binary có sẵn tại bin/xsync. Đang sao chép..."
    MSG_LOCAL_BIN_INSTALLED="Đã cài đặt bản nhị phân có sẵn vào"
    MSG_BUILD_SUCCESS="Đã biên dịch và cài đặt binary thành công!"
    MSG_INSTALL_SUCCESS="Cài đặt xsync thành công tại:"
    MSG_USAGE_TITLE="Cách sử dụng:"
    MSG_USAGE_HINT="Mở một terminal mới (hoặc chạy lệnh dưới) rồi gõ xsync:"
fi

echo ""
echo -e "${C_CYAN}${MSG_HEADER}${C_RESET}"
ok "${MSG_LANG_SAVED}"
echo ""

# 1. Phát hiện hệ điều hành và kiến trúc CPU
OS_RAW="$(uname -s)"
ARCH_RAW="$(uname -m)"

case "${OS_RAW}" in
    Linux*)  OS="linux" ;;
    Darwin*) OS="darwin" ;;
    *)       err "OS ${OS_RAW} is not supported."; exit 1 ;;
esac

case "${ARCH_RAW}" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686)     ARCH="386" ;;
    *)             err "Architecture ${ARCH_RAW} is not supported."; exit 1 ;;
esac

if [ "${CHOSEN_LANG}" = "en" ]; then
    info "Detected operating system: ${OS} (${ARCH})"
else
    info "Phát hiện hệ điều hành: ${OS} (${ARCH})"
fi

# 2. Tự động kiểm tra và cài đặt các phụ thuộc hệ thống (rsync, sshpass)
echo ""
info "${MSG_CHECK_DEPS}"

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
    if [ "${CHOSEN_LANG}" = "en" ]; then
        info "Automatically installing dependencies: ${missing[*]}..."
    else
        info "Đang tự động cài đặt phụ thuộc: ${missing[*]}..."
    fi

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
        warn "Could not identify a compatible package manager."
        return 1
    fi
}

install_darwin_deps() {
    local missing=("$@")
    if command -v brew >/dev/null 2>&1; then
        if [ "${CHOSEN_LANG}" = "en" ]; then
            info "Automatically installing dependencies via Homebrew: ${missing[*]}..."
        else
            info "Đang tự động cài đặt phụ thuộc qua Homebrew: ${missing[*]}..."
        fi
        for dep in "${missing[@]}"; do
            if [ "${dep}" = "rsync" ]; then
                brew install rsync || true
            elif [ "${dep}" = "sshpass" ]; then
                brew install esolitos/ipa/sshpass || brew install hudochenkov/sshpass/sshpass || true
            fi
        done
    else
        warn "Homebrew not found on macOS."
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
    warn "Missing: ${MISSING_DEPS[*]}"
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

    RECHECK_FAILED=0
    for dep in "${MISSING_DEPS[@]}"; do
        if ! command -v "${dep}" >/dev/null 2>&1; then
            RECHECK_FAILED=1
            warn "Could not automatically install '${dep}'. Please install manually."
        fi
    done
    if [ ${RECHECK_FAILED} -eq 0 ]; then
        ok "Dependencies installed successfully!"
    fi
else
    ok "${MSG_ALL_DEPS_READY}"
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

# Chiến lược 1: Sử dụng binary hoặc mã nguồn cục bộ nếu đang ở trong thư mục dự án
if [ -f "bin/xsync" ] && [ "${OS}" = "linux" ] && [ "${ARCH}" = "amd64" ]; then
    info "${MSG_LOCAL_BIN_FOUND}"
    rm -f "${TARGET_PATH}"
    cp bin/xsync "${TARGET_PATH}"
    chmod +x "${TARGET_PATH}"
    ok "${MSG_LOCAL_BIN_INSTALLED} ${TARGET_PATH}!"
    INSTALLED=1
elif [ -f "cmd/xsync/main.go" ] && command -v go >/dev/null 2>&1; then
    rm -f "${TARGET_PATH}"
    go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync
    if [ -f "${TARGET_PATH}" ]; then
        chmod +x "${TARGET_PATH}"
        ok "${MSG_BUILD_SUCCESS}"
        INSTALLED=1
    fi
fi

# Chiến lược 2: Tải pre-built binary từ GitHub Releases (khi chạy qua curl từ xa)
if [ ${INSTALLED} -eq 0 ]; then
    RELEASE_TAR="https://github.com/${REPO}/releases/latest/download/xsync_${OS}_${ARCH}.tar.gz"
    RELEASE_BIN="https://github.com/${REPO}/releases/latest/download/xsync_${OS}_${ARCH}"

    info "${MSG_CHECK_RELEASES}"
    if curl -fsSL -I "${RELEASE_TAR}" >/dev/null 2>&1; then
        if curl -fsSL "${RELEASE_TAR}" | tar -xz -C "${TMP_DIR}" 2>/dev/null; then
            if [ -f "${TMP_DIR}/${BINARY_NAME}" ]; then
                rm -f "${TARGET_PATH}"
                mv "${TMP_DIR}/${BINARY_NAME}" "${TARGET_PATH}"
                chmod +x "${TARGET_PATH}"
                ok "${MSG_RELEASE_INSTALLED}"
                INSTALLED=1
            fi
        fi
    elif curl -fsSL -I "${RELEASE_BIN}" >/dev/null 2>&1; then
        if curl -fsSL "${RELEASE_BIN}" -o "${TARGET_PATH}"; then
            chmod +x "${TARGET_PATH}"
            ok "${MSG_RELEASE_INSTALLED}"
            INSTALLED=1
        fi
    fi
fi

# Chiến lược 3: Tải binary nhánh main từ GitHub nếu chưa có release
if [ ${INSTALLED} -eq 0 ] && [ "${OS}" = "linux" ] && [ "${ARCH}" = "amd64" ]; then
    RAW_URL="https://raw.githubusercontent.com/${REPO}/main/bin/xsync"
    if curl -fsSL "${RAW_URL}" -o "${TARGET_PATH}" 2>/dev/null; then
        chmod +x "${TARGET_PATH}"
        if [ -s "${TARGET_PATH}" ] && head -c 4 "${TARGET_PATH}" | grep -q "ELF"; then
            ok "${MSG_LOCAL_BIN_INSTALLED} ${TARGET_PATH}!"
            INSTALLED=1
        else
            rm -f "${TARGET_PATH}"
        fi
    fi
fi

# Chiến lược 4: Fallback clone và tự biên dịch nếu máy có sẵn Go
if [ ${INSTALLED} -eq 0 ] && command -v go >/dev/null 2>&1; then
    git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src" >/dev/null 2>&1 || true
    if [ -f "${TMP_DIR}/src/cmd/xsync/main.go" ]; then
        (cd "${TMP_DIR}/src" && go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync)
        if [ -f "${TARGET_PATH}" ]; then
            chmod +x "${TARGET_PATH}"
            ok "${MSG_BUILD_SUCCESS}"
            INSTALLED=1
        fi
    fi
fi

if [ ${INSTALLED} -eq 0 ]; then
    err "Unable to download or build xsync binary."
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
                break
            fi
        fi
    done
fi

echo ""
ok "${MSG_INSTALL_SUCCESS} ${TARGET_PATH}"
echo ""
echo -e "${C_CYAN}${MSG_USAGE_TITLE}${C_RESET}"
echo "  ${MSG_USAGE_HINT}"
echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
echo "  xsync"
echo "  xsync --help"
echo ""
