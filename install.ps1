# xsync installer script for Windows PowerShell
# Usage: irm https://raw.githubusercontent.com/SonNX24042005/xsync/main/install.ps1 | iex

param (
    [string]$Lang = ""
)

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

# 0. Lua chon ngon ngu (Language selection)
$ChosenLang = if ($env:XSYNC_LANG) { $env:XSYNC_LANG } else { "" }
if ($Lang -ne "") {
    $ChosenLang = $Lang
}

if (-not $ChosenLang) {
    Write-Host ""
    Write-Host "  Select language / Chon ngon ngu:" -ForegroundColor Cyan
    Write-Host "    1) English (en)" -ForegroundColor White
    Write-Host "    2) Tieng Viet (vi) [Default / Mac dinh]" -ForegroundColor White
    Write-Host ""
    $LangInput = Read-Host "  Enter choice (1-2) [Default: 2]"
    if ($LangInput -eq "1" -or $LangInput -like "*en*") {
        $ChosenLang = "en"
    } else {
        $ChosenLang = "vi"
    }
}

# Luu vao ~/.config/xsync/config.json
$GlobalConfigDir = Join-Path $HOME ".config\xsync"
if (-not (Test-Path $GlobalConfigDir)) {
    New-Item -ItemType Directory -Path $GlobalConfigDir -Force | Out-Null
}
(@{language=$ChosenLang} | ConvertTo-Json -Compress) | Out-File -FilePath (Join-Path $GlobalConfigDir "config.json") -Encoding utf8

if ($ChosenLang -eq "en") {
    $MsgHeader = "INSTALLING XSYNC (PARALLEL RSYNC CLI)"
    $MsgLangSaved = "Default language saved: English (en) in ~/.config/xsync/config.json"
    $MsgCheckRsync = "Checking rsync dependency on Windows..."
    $MsgFoundRsync = "Found rsync:"
    $MsgWarnRsync = "rsync not found. Attempting automatic installation..."
    $MsgReleaseCheck = "Checking pre-built releases on GitHub Releases..."
    $MsgReleaseInstalled = "Downloaded and installed xsync from GitHub Releases successfully!"
    $MsgInstalled = "INSTALLATION COMPLETED!"
    $MsgUsage = "Open a new PowerShell window and run: xsync"
} else {
    $MsgHeader = "CAI DAT XSYNC (PARALLEL RSYNC CLI)"
    $MsgLangSaved = "Da luu ngon ngu mac dinh: Tieng Viet (vi) tai ~/.config/xsync/config.json"
    $MsgCheckRsync = "Dang kiem tra cong cu phu thuoc rsync tren Windows..."
    $MsgFoundRsync = "Da tim thay rsync:"
    $MsgWarnRsync = "Chua tim thay rsync. Dang thu tu dong cai dat..."
    $MsgReleaseCheck = "Dang kiem tra ban phat hanh pre-built tren GitHub Releases..."
    $MsgReleaseInstalled = "Da tai va cai dat xsync tu GitHub Releases thanh cong!"
    $MsgInstalled = "CAI DAT XSYNC HOAN TAT!"
    $MsgUsage = "Mo mot cua so PowerShell moi va chay: xsync"
}

Write-Host ""
Write-Host "======================================================" -ForegroundColor Magenta
Write-Host "         $MsgHeader" -ForegroundColor Magenta
Write-Host "======================================================" -ForegroundColor Magenta
Write-Ok $MsgLangSaved
Write-Host ""

# 1. Kiem tra va tu dong cai dat phu thuoc rsync
Write-Info $MsgCheckRsync
if (Get-Command rsync -ErrorAction SilentlyContinue) {
    Write-Ok "$MsgFoundRsync $((Get-Command rsync).Source)"
} else {
    Write-Warn $MsgWarnRsync
    $RsyncInstalled = $false

    # Thu qua Scoop
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        Write-Info "Installing rsync via Scoop..."
        try {
            scoop install rsync
            if (Get-Command rsync -ErrorAction SilentlyContinue) {
                Write-Ok "rsync installed via Scoop successfully!"
                $RsyncInstalled = $true
            }
        } catch {
            Write-Warn "Scoop install failed: $_"
        }
    }

    # Thu qua Chocolatey
    if (-not $RsyncInstalled -and (Get-Command choco -ErrorAction SilentlyContinue)) {
        Write-Info "Installing rsync via Chocolatey..."
        try {
            choco install rsync -y
            if (Get-Command rsync -ErrorAction SilentlyContinue) {
                Write-Ok "rsync installed via Chocolatey successfully!"
                $RsyncInstalled = $true
            }
        } catch {
            Write-Warn "Chocolatey install failed: $_"
        }
    }

    if (-not $RsyncInstalled) {
        Write-Warn "Could not automatically install rsync."
        Write-Host "  Install Scoop and run: scoop install rsync" -ForegroundColor Yellow
        Write-Host "  Or use Git Bash / WSL." -ForegroundColor Yellow
    }
}

# 2. Tao thu muc cai dat
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TargetPath = Join-Path $InstallDir $BinaryName
$TempDir = Join-Path $env:TEMP "xsync_install_$([System.Guid]::NewGuid().ToString('N'))"
$Installed = $false

try {
    # Chien luoc 1: Tai pre-built binary tu GitHub Releases
    $ReleaseZipUrl = "https://github.com/$Repo/releases/latest/download/xsync_windows_amd64.zip"
    $ReleaseExeUrl = "https://github.com/$Repo/releases/latest/download/xsync_windows_amd64.exe"

    Write-Info $MsgReleaseCheck
    $ZipPath = Join-Path $env:TEMP "xsync_windows_amd64.zip"

    try {
        Invoke-WebRequest -Uri $ReleaseZipUrl -OutFile $ZipPath -ErrorAction Stop
        if (Test-Path $ZipPath) {
            New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
            Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
            $ExtractedExe = Join-Path $TempDir $BinaryName
            if (Test-Path $ExtractedExe) {
                if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
                Move-Item -Path $ExtractedExe -Destination $TargetPath -Force
                Write-Ok $MsgReleaseInstalled
                $Installed = $true
            }
            Remove-Item -Path $ZipPath -Force -ErrorAction SilentlyContinue
        }
    } catch {
        try {
            Invoke-WebRequest -Uri $ReleaseExeUrl -OutFile $TargetPath -ErrorAction Stop
            Write-Ok $MsgReleaseInstalled
            $Installed = $true
        } catch {
            Write-Info "Checking local or source fallback..."
        }
    }

    # Chien luoc 2: Su dung file binary co san trong thu muc repo neu chay cuc bo
    if (-not $Installed -and (Test-Path "bin\xsync.exe")) {
        if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
        Copy-Item -Path "bin\xsync.exe" -Destination $TargetPath -Force
        Write-Ok "Copied local pre-built binary successfully!"
        $Installed = $true
    }

    # Chien luoc 3: Fallback bien dich tu ma nguon neu co Go compiler
    if (-not $Installed -and (Get-Command go -ErrorAction SilentlyContinue)) {
        if (Test-Path "cmd/xsync/main.go") {
            if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
            go build -ldflags="-s -w" -o $TargetPath ./cmd/xsync
            $Installed = $true
        } else {
            New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
            git clone --depth 1 "https://github.com/${Repo}.git" $TempDir
            if (Test-Path (Join-Path $TempDir "cmd/xsync/main.go")) {
                Set-Location $TempDir
                if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
                go build -ldflags="-s -w" -o $TargetPath ./cmd/xsync
                $Installed = $true
            }
        }
        if ($Installed) {
            Write-Ok "Compiled and installed xsync successfully!"
        }
    }

    if (-not $Installed) {
        Write-Err "Could not download or build xsync."
        exit 1
    }
} finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}

# 3. Cap nhat bien moi truong PATH
$UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [System.Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Ok "Added xsync to PATH!"
}

Write-Host "======================================================" -ForegroundColor Green
Write-Host "         $MsgInstalled                                " -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Green
Write-Host "  $MsgUsage" -ForegroundColor Cyan
Write-Host ""
