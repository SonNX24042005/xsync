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

# 1. Kiem tra kien truc may
$Arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
$ZipName = "xsync_windows_${Arch}.zip"
$ReleaseUrl = "https://github.com/${Repo}/releases/latest/download/${ZipName}"

# 2. Tao thu muc cai dat
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TargetPath = Join-Path $InstallDir $BinaryName
$TempDir = Join-Path $env:TEMP "xsync_install_$([System.Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

$Installed = $false

# 3. Tai pre-built binary tu GitHub Releases
try {
    Write-Info "Dang tai pre-built binary tu GitHub Releases: $ReleaseUrl ..."
    $ZipPath = Join-Path $TempDir $ZipName
    Invoke-WebRequest -Uri $ReleaseUrl -OutFile $ZipPath -UseBasicParsing
    
    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
    $ExtractedExe = Join-Path $TempDir "xsync.exe"
    if (-not (Test-Path $ExtractedExe)) {
        $ExtractedExe = Join-Path $TempDir "xsync"
    }

    if (Test-Path $ExtractedExe) {
        if (Test-Path $TargetPath) {
            Remove-Item $TargetPath -Force
        }
        Move-Item -Path $ExtractedExe -Destination $TargetPath -Force
        $Installed = $true
        Write-Ok "Da tai va cai dat binary thanh cong vao: $TargetPath"
    }
} catch {
    Write-Warn "Khong the tai pre-built binary truc tiep: $_"
} finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}

# 4. Fallback: Bien dich tu ma nguon neu co Go compiler
if (-not $Installed) {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Info "Phat hien Go compiler tren may. Dang bien dich tu ma nguon..."
        $SrcTemp = Join-Path $env:TEMP "xsync_src_$([System.Guid]::NewGuid().ToString('N'))"
        try {
            git clone --depth 1 "https://github.com/${Repo}.git" $SrcTemp
            Set-Location $SrcTemp
            go build -ldflags="-s -w" -o $TargetPath ./cmd/xsync
            $Installed = $true
            Write-Ok "Da bien dich va cai dat xsync thanh cong vao: $TargetPath"
        } finally {
            Remove-Item -Path $SrcTemp -Recurse -Force -ErrorAction SilentlyContinue
        }
    } else {
        Write-Err "Cai dat that bai. Vui long kiem tra lai ket noi mang hoac cai dat Go compiler."
        exit 1
    }
}

# 5. Cap nhat bien moi truong PATH
$UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Info "Dang them '$InstallDir' vao bien moi truong PATH..."
    [System.Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Ok "Da them xsync vao PATH thanh cong!"
}

# 6. Kiem tra phu thuoc rsync tren Windows
Write-Host ""
Write-Info "Dang kiem tra cong cu rsync tren Windows..."
if (Get-Command rsync -ErrorAction SilentlyContinue) {
    Write-Ok "Da tim thay rsync tren he thong: $((Get-Command rsync).Source)"
} else {
    Write-Warn "Chua tim thay cong cu 'rsync' trong PATH."
    Write-Host ""
    Write-Host "  De su dung xsync tren Windows, ban co the chon mot trong cac cach sau:" -ForegroundColor Yellow
    Write-Host "  1. Cai dat rsync qua Scoop: (Khuyen nghi)" -ForegroundColor Cyan
    Write-Host "       scoop install rsync" -ForegroundColor White
    Write-Host "  2. Su dung ben trong Git Bash (da co san SSH & ho tro cai rsync)" -ForegroundColor Cyan
    Write-Host "  3. Su dung thong qua WSL (Windows Subsystem for Linux)" -ForegroundColor Cyan
    Write-Host ""
}

Write-Host "======================================================" -ForegroundColor Green
Write-Host "         CAI DAT XSYNC HOAN TAT!                      " -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Green
Write-Host "  Mo mot cua so Terminal/PowerShell moi va go lenh:" -ForegroundColor White
Write-Host "  > xsync" -ForegroundColor Cyan
Write-Host ""
