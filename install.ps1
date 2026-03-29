# miraeboy / miraeboy-agent Windows installer
#
# 사용법:
#   irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install.ps1 | iex
#   iex "& { $(irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install.ps1) } --agent"
#   iex "& { $(irm .../install.ps1) } --version v1.2.0"
#
# 환경변수:
#   $env:MIRAEBOY_VERSION      설치할 버전 (기본값: latest)
#   $env:MIRAEBOY_INSTALL_DIR  설치 경로  (기본값: $env:ProgramFiles\miraeboy)
#   $env:MIRAEBOY_BASE_URL     릴리즈 base URL
#
[CmdletBinding()]
param(
    [switch]$Agent,
    [string]$Version  = $env:MIRAEBOY_VERSION,
    [string]$InstallDir = $env:MIRAEBOY_INSTALL_DIR,
    [string]$BaseUrl  = $env:MIRAEBOY_BASE_URL
)

$ErrorActionPreference = 'Stop'

# ── 설정 ──────────────────────────────────────────────────────────────────────

$Repo    = "corbie79/miraeboy"
$Binary  = if ($Agent) { "miraeboy-agent" } else { "miraeboy" }
if (-not $BaseUrl) { $BaseUrl = "https://github.com/$Repo" }

# ── 아키텍처 감지 ─────────────────────────────────────────────────────────────

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default {
        Write-Error "지원하지 않는 아키텍처: $env:PROCESSOR_ARCHITECTURE"
        exit 1
    }
}

# ── 최신 버전 조회 ────────────────────────────────────────────────────────────

if (-not $Version) {
    Write-Host "==> Fetching latest release..." -NoNewline
    try {
        $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $rel.tag_name
    } catch {
        Write-Host " FAILED"
        Write-Error "버전 조회 실패. --Version vX.Y.Z 를 직접 지정하세요."
        exit 1
    }
    Write-Host " $Version"
}

# ── 다운로드 URL ──────────────────────────────────────────────────────────────

$Archive     = "$Binary-windows-$Arch-$Version.zip"
$DownloadUrl = "$BaseUrl/releases/download/$Version/$Archive"
$ChecksumUrl = "$BaseUrl/releases/download/$Version/checksums.txt"

# ── 설치 경로 결정 ────────────────────────────────────────────────────────────

if (-not $InstallDir) {
    $InstallDir = "$env:ProgramFiles\miraeboy"
}

# ── 다운로드 ──────────────────────────────────────────────────────────────────

$TmpDir = Join-Path $env:TEMP "miraeboy-install-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    Write-Host "==> Installing $Binary $Version (windows/$Arch)"
    Write-Host "==> Downloading $Archive..."
    $ArchivePath = Join-Path $TmpDir $Archive
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing

    # ── 체크섬 검증 ────────────────────────────────────────────────────────────

    try {
        $ChecksumPath = Join-Path $TmpDir "checksums.txt"
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath -UseBasicParsing -ErrorAction Stop

        Write-Host "==> Verifying checksum..." -NoNewline
        $expected = (Get-Content $ChecksumPath | Where-Object { $_ -match "  $Archive$" }) -replace "  $Archive$", "" -replace "\s", ""
        $actual   = (Get-FileHash $ArchivePath -Algorithm SHA256).Hash.ToLower()

        if ($expected -and $expected -eq $actual) {
            Write-Host " OK"
        } elseif ($expected) {
            Write-Host " FAILED"
            Write-Error "체크섬 불일치! 다운로드가 손상되었을 수 있습니다."
            exit 1
        } else {
            Write-Host " SKIPPED (not found in checksums.txt)"
        }
    } catch {
        Write-Host "==> Checksum file not found, skipping verification"
    }

    # ── 압축 해제 ──────────────────────────────────────────────────────────────

    Write-Host "==> Extracting..."
    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force

    $ExeName  = "$Binary-windows-$Arch.exe"
    $ExtractedPath = Join-Path $TmpDir $ExeName

    if (-not (Test-Path $ExtractedPath)) {
        Write-Error "압축 해제 후 바이너리를 찾을 수 없습니다: $ExeName"
        exit 1
    }

    # ── 설치 ───────────────────────────────────────────────────────────────────

    if (-not (Test-Path $InstallDir)) {
        Write-Host "==> Creating $InstallDir..."
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    $Dest = Join-Path $InstallDir "$Binary.exe"
    Write-Host "==> Installing to $Dest..."

    # 실행 중인 경우 대비해 기존 파일 삭제
    if (Test-Path $Dest) { Remove-Item $Dest -Force }
    Move-Item $ExtractedPath $Dest

    # ── PATH 등록 ──────────────────────────────────────────────────────────────

    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
    if ($currentPath -notlike "*$InstallDir*") {
        Write-Host "==> Adding $InstallDir to system PATH..."
        try {
            [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallDir", "Machine")
            $env:PATH = "$env:PATH;$InstallDir"
            Write-Host "    PATH 업데이트 완료 (새 터미널에서 적용됨)"
        } catch {
            Write-Warning "PATH 자동 등록 실패 (관리자 권한 필요). 수동으로 추가하세요:"
            Write-Warning "  $InstallDir"
        }
    }

    # ── 완료 ───────────────────────────────────────────────────────────────────

    Write-Host ""
    Write-Host "  OK  $Binary $Version installed -> $Dest" -ForegroundColor Green
    Write-Host ""

    if ($Binary -eq "miraeboy") {
        Write-Host "  Quick start:"
        Write-Host "    miraeboy.exe --help"
        Write-Host "    # config.yaml 편집 후:"
        Write-Host "    miraeboy.exe"
    } else {
        Write-Host "  Quick start:"
        Write-Host "    miraeboy-agent.exe --server http://miraeboy.example.com:9300 --agent-key YOUR_KEY"
    }

} finally {
    Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
