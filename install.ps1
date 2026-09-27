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

# 1. Kiem tra va tu dong cai dat phu thuoc rsync
Write-Info "Dang kiem tra cong cu phu thuoc rsync tren Windows..."
if (Get-Command rsync -ErrorAction SilentlyContinue) {
    Write-Ok "Da tim thay rsync: $((Get-Command rsync).Source)"
} else {
    Write-Warn "Chua tim thay rsync. Dang thu tu dong cai dat..."
    $RsyncInstalled = $false

    # Thu qua Scoop
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        Write-Info "Dang cai dat rsync qua Scoop..."
        try {
            scoop install rsync
            if (Get-Command rsync -ErrorAction SilentlyContinue) {
                Write-Ok "Da cai dat rsync qua Scoop thanh cong!"
                $RsyncInstalled = $true
            }
        } catch {
            Write-Warn "Cai dat qua Scoop gap loi: $_"
        }
    }

    # Thu qua Chocolatey
    if (-not $RsyncInstalled -and (Get-Command choco -ErrorAction SilentlyContinue)) {
        Write-Info "Dang cai dat rsync qua Chocolatey..."
        try {
            choco install rsync -y
            if (Get-Command rsync -ErrorAction SilentlyContinue) {
                Write-Ok "Da cai dat rsync qua Chocolatey thanh cong!"
                $RsyncInstalled = $true
            }
        } catch {
            Write-Warn "Cai dat qua Chocolatey gap loi: $_"
        }
    }

    if (-not $RsyncInstalled) {
        Write-Warn "Chua the tu dong cai dat rsync."
        Write-Host "  De su dung, ban co the cai dat Scoop va chay: scoop install rsync" -ForegroundColor Yellow
        Write-Host "  Hoac su dung Git Bash / WSL co san rsync." -ForegroundColor Yellow
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

    Write-Info "Dang kiem tra ban phat hanh pre-built tren GitHub Releases..."
    $ZipPath = Join-Path $env:TEMP "xsync_windows_amd64.zip"

    try {
        Invoke-WebRequest -Uri $ReleaseZipUrl -OutFile $ZipPath -ErrorAction Stop
        if (Test-Path $ZipPath) {
            Write-Info "Dang giai nen ban binary..."
            New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
            Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
            $ExtractedExe = Join-Path $TempDir $BinaryName
            if (Test-Path $ExtractedExe) {
                if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
                Move-Item -Path $ExtractedExe -Destination $TargetPath -Force
                Write-Ok "Da tai va cai dat xsync tu GitHub Releases thanh cong!"
                $Installed = $true
            }
            Remove-Item -Path $ZipPath -Force -ErrorAction SilentlyContinue
        }
    } catch {
        try {
            Invoke-WebRequest -Uri $ReleaseExeUrl -OutFile $TargetPath -ErrorAction Stop
            Write-Ok "Da tai ban nhan xsync.exe truc tiep tu GitHub Releases thanh cong!"
            $Installed = $true
        } catch {
            Write-Info "Chua co ban release tren GitHub, chuyen sang phuong an tiep theo..."
        }
    }

    # Chien luoc 2: Su dung file binary co san trong thu muc repo neu chay cuc bo
    if (-not $Installed -and (Test-Path "bin\xsync.exe")) {
        Write-Info "Phat hien ban binary co san tai bin\xsync.exe. Dang sao chep..."
        if (Test-Path $TargetPath) { Remove-Item $TargetPath -Force }
        Copy-Item -Path "bin\xsync.exe" -Destination $TargetPath -Force
        Write-Ok "Da sao chep ban binary co san thanh cong!"
        $Installed = $true
    }

    # Chien luoc 3: Fallback bien dich tu ma nguon neu co Go compiler
    if (-not $Installed -and (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Info "Phat hien Go compiler: $((Get-Command go).Source). Dang bien dich..."
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
            Write-Ok "Da bien dich va cai dat xsync thanh cong tai: $TargetPath"
        }
    }

    if (-not $Installed) {
        Write-Err "Khong the tu dong tai ban nhị phan hoac bien dich xsync."
        Write-Host ""
        Write-Host "  Vui long thu mot trong cac cach sau:" -ForegroundColor Yellow
        Write-Host "    1. Cai dat Go compiler: scoop install go hoac winget install GoLang.Go" -ForegroundColor White
        Write-Host "    2. Tai file zip binary tu: https://github.com/$Repo/releases" -ForegroundColor White
        Write-Host ""
        exit 1
    }
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

Write-Host "======================================================" -ForegroundColor Green
Write-Host "         CAI DAT XSYNC HOAN TAT!                      " -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Green
Write-Host "  Mo mot cua so Terminal/PowerShell moi va go lenh:" -ForegroundColor White
Write-Host "  > xsync" -ForegroundColor Cyan
Write-Host ""
