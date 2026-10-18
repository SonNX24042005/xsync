#!/usr/bin/env bash
# ==============================================================================
# Script cài đặt bản phát hành xsync mới nhất
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
    MSG_ALL_DEPS_READY="All dependencies (rsync, sshpass, git) are ready!"
    MSG_CHECK_RELEASES="Checking releases on GitHub Releases..."
    MSG_RELEASE_INSTALLED="Installed binary from GitHub Releases successfully!"
    MSG_BUILD_SUCCESS="Compiled and installed binary successfully!"
    MSG_INSTALL_SUCCESS="xsync installed successfully at:"
    MSG_USAGE_TITLE="How to use:"
    MSG_USAGE_HINT="Open a new terminal (or run below) and type xsync:"
else
    MSG_HEADER="=== Cài đặt công cụ xsync ==="
    MSG_LANG_SAVED="Đã lưu cấu hình ngôn ngữ: Tiếng Việt (vi) tại ~/.config/xsync/config.json"
    MSG_CHECK_DEPS="Kiểm tra các công cụ phụ thuộc hệ thống..."
    MSG_ALL_DEPS_READY="Tất cả phụ thuộc (rsync, sshpass, git) đã sẵn sàng!"
    MSG_CHECK_RELEASES="Kiểm tra bản phát hành trên GitHub Releases..."
    MSG_RELEASE_INSTALLED="Đã cài đặt bản nhị phân từ GitHub Releases thành công!"
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

# 2. Tự động kiểm tra và cài đặt các phụ thuộc hệ thống (rsync, sshpass, git)
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
            if [ "${dep}" = "rsync" ] || [ "${dep}" = "git" ]; then
                brew install "${dep}" || true
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
if ! command -v git >/dev/null 2>&1; then
    MISSING_DEPS+=("git")
fi
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
    if [ -n "${STAGED_PATH:-}" ]; then
        rm -f "${STAGED_PATH}"
    fi
}
trap cleanup EXIT

INSTALLED=0

# 4. Ưu tiên bản phát hành, không dùng binary cũ trong thư mục hiện tại.
echo ""

STAGED_PATH=""
install_binary() {
    local candidate="$1"
    chmod +x "${candidate}" || return 1
    if ! "${candidate}" --version >/dev/null 2>&1; then
        warn "Downloaded or built executable cannot run on this machine."
        return 1
    fi
    STAGED_PATH="$(mktemp "${INSTALL_DIR}/.xsync-install.XXXXXX")" || return 1
    if ! cp "${candidate}" "${STAGED_PATH}" || ! chmod 755 "${STAGED_PATH}" || ! mv -f "${STAGED_PATH}" "${TARGET_PATH}"; then
        rm -f "${STAGED_PATH}"
        STAGED_PATH=""
        return 1
    fi
    STAGED_PATH=""
    INSTALLED=1
}

RELEASE_TAR="https://github.com/${REPO}/releases/latest/download/xsync_${OS}_${ARCH}.tar.gz"
RELEASE_BIN="https://github.com/${REPO}/releases/latest/download/xsync_${OS}_${ARCH}"
info "${MSG_CHECK_RELEASES}"
if curl -fsSL --retry 2 "${RELEASE_TAR}" -o "${TMP_DIR}/release.tar.gz"; then
    if tar -xzf "${TMP_DIR}/release.tar.gz" -C "${TMP_DIR}" "${BINARY_NAME}" && [ -f "${TMP_DIR}/${BINARY_NAME}" ]; then
        if install_binary "${TMP_DIR}/${BINARY_NAME}"; then
            ok "${MSG_RELEASE_INSTALLED}"
        fi
    fi
fi
if [ ${INSTALLED} -eq 0 ]; then
    if curl -fsSL --retry 2 "${RELEASE_BIN}" -o "${TMP_DIR}/release-binary"; then
        if install_binary "${TMP_DIR}/release-binary"; then
            ok "${MSG_RELEASE_INSTALLED}"
        fi
    fi
fi

# Chỉ build từ nguồn khi không tải được release và đã có Go.
if [ ${INSTALLED} -eq 0 ] && command -v go >/dev/null 2>&1; then
    if [ -f "cmd/xsync/main.go" ]; then
        if go build -ldflags="-s -w" -o "${TMP_DIR}/built-xsync" ./cmd/xsync; then
            if install_binary "${TMP_DIR}/built-xsync"; then
                ok "${MSG_BUILD_SUCCESS}"
            fi
        fi
    elif command -v git >/dev/null 2>&1; then
        if git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src"; then
            if (cd "${TMP_DIR}/src" && go build -ldflags="-s -w" -o "${TMP_DIR}/built-xsync" ./cmd/xsync); then
                if install_binary "${TMP_DIR}/built-xsync"; then
                    ok "${MSG_BUILD_SUCCESS}"
                fi
            fi
        fi
    fi
fi

if [ ${INSTALLED} -eq 0 ]; then
    err "Unable to download or build xsync. The existing installation has been preserved."
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
"${TARGET_PATH}" --version
RESOLVED_XSYNC="$(command -v xsync || true)"
if [ -n "${RESOLVED_XSYNC}" ] && [ "${RESOLVED_XSYNC}" != "${TARGET_PATH}" ]; then
    warn "Another xsync is earlier on PATH: ${RESOLVED_XSYNC}. Use the PATH command below."
fi
echo ""
echo -e "${C_CYAN}${MSG_USAGE_TITLE}${C_RESET}"
echo "  ${MSG_USAGE_HINT}"
echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
echo "  hash -r"
echo "  xsync"
echo "  xsync --help"
echo ""
