# xsync installer script for Windows PowerShell
# Usage: irm https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = if ($env:XSYNC_REPO) { $env:XSYNC_REPO } else { "SonNX24042005/xsync" }
$BinaryName = "xsync.exe"
$InstallDir = Join-Path $HOME "AppData\Local\Programs\xsync"

function Write-Info($msg) {
    Write-Host "  [INFO] $msg" -ForegroundColor Cyan
}

function Write-Ok($msg) {
    Write-Host "  [OK]   $msg" -ForegroundColor Green
}

function Write-Warn($msg) {
    Write-Host "  [WARN] $msg" -ForegroundColor Yellow
}

function Write-Err($msg) {
    Write-Host "  [ERR]  $msg" -ForegroundColor Red
}

Write-Host ""
Write-Host "======================================================" -ForegroundColor Magenta
Write-Host "         CAI DAT XSYNC (PARALLEL RSYNC CLI)           " -ForegroundColor Magenta
Write-Host "======================================================" -ForegroundColor Magenta
Write-Host ""

# 1. Kiem tra Go compiler
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Err "Khong tim thay Go compiler tren may Windows."
    Write-Host ""
    Write-Host "  Vui long cai dat Go de bien dich:" -ForegroundColor Yellow
    Write-Host "    - Qua Scoop:  scoop install go" -ForegroundColor White
    Write-Host "    - Qua Winget: winget install GoLang.Go" -ForegroundColor White
    Write-Host "    - Hoac tai:   https://go.dev/dl/" -ForegroundColor White
    Write-Host ""
    exit 1
}

Write-Info "Phat hien Go compiler: $((Get-Command go).Source)"

# 2. Tao thu muc cai dat
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TargetPath = Join-Path $InstallDir $BinaryName
$TempDir = Join-Path $env:TEMP "xsync_install_$([System.Guid]::NewGuid().ToString('N'))"

try {
    if (Test-Path "cmd/xsync/main.go") {
        Write-Info "Dang bien dich tu ma nguon thu muc hien tai..."
        if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
        go build -ldflags="-s -w" -o $TargetPath ./cmd/xsync
    } else {
        Write-Info "Dang clone ma nguon tu GitHub..."
        git clone --depth 1 "https://github.com/${Repo}.git" $TempDir
        if (Test-Path (Join-Path $TempDir "cmd/xsync/main.go")) {
            Write-Info "Dang bien dich binary xsync..."
            Set-Location $TempDir
            if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
            go build -ldflags="-s -w" -o $TargetPath ./cmd/xsync
        } else {
            Write-Err "Khong the clone ma nguon tu repository."
            exit 1
        }
    }
    Write-Ok "Da bien dich va cai dat xsync thanh cong tai: $TargetPath"
} finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}

# 3. Cap nhat bien moi truong PATH
$UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Info "Dang them '$InstallDir' vao bien moi truong PATH..."
    [System.Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Ok "Da them xsync vao PATH thanh cong!"
}

# 4. Kiem tra phu thuoc rsync
Write-Host ""
Write-Info "Dang kiem tra cong cu rsync tren Windows..."
if (Get-Command rsync -ErrorAction SilentlyContinue) {
    Write-Ok "Da tim thay rsync: $((Get-Command rsync).Source)"
} else {
    Write-Warn "Chua tim thay cong cu 'rsync' trong PATH."
    Write-Host ""
    Write-Host "  De su dung tren Windows, ban co the cai dat rsync qua Scoop:" -ForegroundColor Yellow
    Write-Host "    scoop install rsync" -ForegroundColor White
    Write-Host ""
}

Write-Host "======================================================" -ForegroundColor Green
Write-Host "         CAI DAT XSYNC HOAN TAT!                      " -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Green
Write-Host "  Mo mot cua so Terminal/PowerShell moi va go lenh:" -ForegroundColor White
Write-Host "  > xsync" -ForegroundColor Cyan
Write-Host ""
