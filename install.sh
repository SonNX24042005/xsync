#!/usr/bin/env bash
set -e

# ==============================================================================
# xsync Installer Script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.sh | bash
# Or locally:
#   ./install.sh
# ==============================================================================

REPO="${XSYNC_REPO:-"SonNX24042005/xsync"}"
INSTALL_DIR="${INSTALL_DIR:-"$HOME/.local/bin"}"
BINARY_NAME="xsync"

# ANSI Colors
C_RESET="\033[0m"
C_INFO="\033[36m"
C_OK="\033[32m"
C_WARN="\033[33m"
C_ERR="\033[31m"
C_BOLD="\033[1m"

info()  { echo -e "  ${C_INFO}[INFO]${C_RESET} $*"; }
ok()    { echo -e "  ${C_OK}[OK]${C_RESET}   $*"; }
warn()  { echo -e "  ${C_WARN}[WARN]${C_RESET} $*"; }
err()   { echo -e "  ${C_ERR}[ERR]${C_RESET}  $*"; }

echo -e "\n${C_BOLD}=== Cai dat cong cu xsync ===${C_RESET}\n"

# 1. Detect OS & Architecture
OS_RAW="$(uname -s)"
ARCH_RAW="$(uname -m)"

case "${OS_RAW}" in
    Linux*)  OS="linux" ;;
    Darwin*) OS="darwin" ;;
    *)       err "He dieu hanh ${OS_RAW} chua duoc ho tro."; exit 1 ;;
esac

case "${ARCH_RAW}" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    i386|i686)     ARCH="386" ;;
    *)             err "Kien truc ${ARCH_RAW} chua duoc ho tro."; exit 1 ;;
esac

info "Phat hien he dieu hanh: ${OS} (${ARCH})"

mkdir -p "${INSTALL_DIR}"
TARGET_PATH="${INSTALL_DIR}/${BINARY_NAME}"
INSTALLED=0

# 2. Strategy 1: Download pre-built binary from GitHub Releases
RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
info "Thu tai pre-built binary tu GitHub Releases..."

TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

if curl -fsSL -I "${RELEASE_URL}" > /dev/null 2>&1; then
    info "Dang tai tu: ${RELEASE_URL}"
    if curl -fsSL "${RELEASE_URL}" | tar -xz -C "${TMP_DIR}" 2>/dev/null; then
        if [ -f "${TMP_DIR}/${BINARY_NAME}" ]; then
            rm -f "${TARGET_PATH}"
            mv "${TMP_DIR}/${BINARY_NAME}" "${TARGET_PATH}"
            chmod +x "${TARGET_PATH}"
            ok "Da tai va cai dat binary tu GitHub Releases thanh cong!"
            INSTALLED=1
        fi
    fi
fi

# 3. Strategy 2: Fallback to build from source via Go compiler
if [ ${INSTALLED} -eq 0 ]; then
    warn "Chua co pre-built release tren GitHub. Chuyen sang che do build tu ma nguon..."
    if command -v go >/dev/null 2>&1; then
        info "Phat hien Go compiler: $(go version)"
        
        # Check if running inside local source repo
        rm -f "${TARGET_PATH}"
        if [ -f "cmd/xsync/main.go" ]; then
            info "Dang bien dich tu thu muc ma nguon hien tai..."
            go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync
        else
            info "Dang clone ma nguon tu repository..."
            git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src" >/dev/null 2>&1 || true
            if [ -f "${TMP_DIR}/src/cmd/xsync/main.go" ]; then
                (cd "${TMP_DIR}/src" && go build -ldflags="-s -w" -o "${TARGET_PATH}" ./cmd/xsync)
            else
                err "Khong the clone ma nguon. Vui long kiem tra repository URL hoac bien moi truong XSYNC_REPO."
                exit 1
            fi
        fi
        chmod +x "${TARGET_PATH}"
        ok "Da bien dich va cai dat binary thanh cong!"
        INSTALLED=1
    else
        err "Khong tim thay Go compiler de bien dich tu ma nguon."
        echo -e "\nVui long cai dat Go (https://go.dev/dl/) hoac kiem tra lai release binary.\n"
        exit 1
    fi
fi

# 4. Configure PATH in Shell Profile
SHELL_NAME="$(basename "${SHELL:-bash}")"
RC_FILES=()

case "${SHELL_NAME}" in
    bash) RC_FILES+=("${HOME}/.bashrc") ;;
    zsh)  RC_FILES+=("${HOME}/.zshrc") ;;
    fish) RC_FILES+=("${HOME}/.config/fish/config.fish") ;;
esac
RC_FILES+=("${HOME}/.profile")

PATH_EXPORT="export PATH=\"\$PATH:${INSTALL_DIR}\""

for rc in "${RC_FILES[@]}"; do
    if [ -f "${rc}" ]; then
        if ! grep -q "${INSTALL_DIR}" "${rc}" 2>/dev/null; then
            echo -e "\n# Added by xsync installer\n${PATH_EXPORT}" >> "${rc}"
            ok "Da them ${INSTALL_DIR} vao PATH trong ${rc}"
        fi
    fi
done

# 5. Check System Dependencies (rsync, sshpass)
echo ""
info "Kiem tra cac cong cu phu thuoc..."
MISSING_DEPS=()
if ! command -v rsync >/dev/null 2>&1; then
    MISSING_DEPS+=("rsync")
fi
if ! command -v sshpass >/dev/null 2>&1; then
    MISSING_DEPS+=("sshpass")
fi

if [ ${#MISSING_DEPS[@]} -gt 0 ]; then
    warn "He thong con thieu: ${MISSING_DEPS[*]}"
    if [ "${OS}" = "linux" ]; then
        info "Cai dat bang lenh: sudo apt update && sudo apt install -y ${MISSING_DEPS[*]}"
    elif [ "${OS}" = "darwin" ]; then
        info "Cai dat bang lenh: brew install ${MISSING_DEPS[*]}"
    fi
else
    ok "Tat ca phu thuoc (rsync, sshpass) da san sang!"
fi

# 6. Verification
echo ""
if [ -x "${TARGET_PATH}" ]; then
    ok "Cai dat xsync thanh cong tai: ${TARGET_PATH}"
    echo -e "\n${C_BOLD}Cach su dung:${C_RESET}"
    echo -e "  Mo mot terminal moi (hoac chay lenh duoi) roi go ${C_INFO}xsync${C_RESET}:"
    echo -e "  ${C_BOLD}export PATH=\"\$PATH:${INSTALL_DIR}\"${C_RESET}\n"
else
    err "Khong tim thay file binary sau khi cai dat."
    exit 1
fi
