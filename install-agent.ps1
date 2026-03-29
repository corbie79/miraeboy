# miraeboy-agent Windows installer
#
# 사용법:
#   irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.ps1 | iex
#   iex "& { $(irm .../install-agent.ps1) } -Version v1.2.0"
#   iex "& { $(irm .../install-agent.ps1) } -InstallDir $env:USERPROFILE\.local\bin"
#
# 환경변수:
#   $env:MIRAEBOY_VERSION      설치할 버전 (기본값: latest)
#   $env:MIRAEBOY_INSTALL_DIR  설치 경로  (기본값: $env:ProgramFiles\miraeboy)
#   $env:MIRAEBOY_BASE_URL     릴리즈 base URL
#
[CmdletBinding()]
param(
    [string]$Version    = $env:MIRAEBOY_VERSION,
    [string]$InstallDir = $env:MIRAEBOY_INSTALL_DIR,
    [string]$BaseUrl    = $env:MIRAEBOY_BASE_URL
)

$ErrorActionPreference = 'Stop'

$Repo   = "corbie79/miraeboy"
$Binary = "miraeboy-agent"
if (-not $BaseUrl) { $BaseUrl = "https://github.com/$Repo" }

$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { Write-Error "지원하지 않는 아키텍처: $env:PROCESSOR_ARCHITECTURE"; exit 1 }
}

if (-not $Version) {
    Write-Host "==> Fetching latest release..." -NoNewline
    try {
        $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $rel.tag_name
    } catch {
        Write-Host " FAILED"
        Write-Error "버전 조회 실패. -Version vX.Y.Z 를 직접 지정하세요."
        exit 1
    }
    Write-Host " $Version"
}

$Archive     = "$Binary-windows-$Arch-$Version.zip"
$DownloadUrl = "$BaseUrl/releases/download/$Version/$Archive"
$ChecksumUrl = "$BaseUrl/releases/download/$Version/checksums.txt"

if (-not $InstallDir) { $InstallDir = "$env:ProgramFiles\miraeboy" }

$TmpDir = Join-Path $env:TEMP "miraeboy-agent-install-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    Write-Host "==> Installing miraeboy-agent $Version (windows/$Arch)"
    Write-Host "==> Downloading $Archive..."
    $ArchivePath = Join-Path $TmpDir $Archive
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing

    try {
        $ChecksumPath = Join-Path $TmpDir "checksums.txt"
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath -UseBasicParsing -ErrorAction Stop
        Write-Host "==> Verifying checksum..." -NoNewline
        $expected = (Get-Content $ChecksumPath | Where-Object { $_ -match "  $Archive$" }) -replace "  $Archive$", "" -replace "\s", ""
        $actual   = (Get-FileHash $ArchivePath -Algorithm SHA256).Hash.ToLower()
        if ($expected -and $expected -eq $actual) { Write-Host " OK" }
        elseif ($expected) { Write-Host " FAILED"; Write-Error "체크섬 불일치!"; exit 1 }
        else { Write-Host " SKIPPED" }
    } catch { Write-Host "==> Checksum file not found, skipping verification" }

    Write-Host "==> Extracting..."
    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force
    $ExtractedPath = Join-Path $TmpDir "$Binary-windows-$Arch.exe"
    if (-not (Test-Path $ExtractedPath)) { Write-Error "바이너리를 찾을 수 없습니다."; exit 1 }

    if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }
    $Dest = Join-Path $InstallDir "miraeboy-agent.exe"
    Write-Host "==> Installing to $Dest..."
    if (Test-Path $Dest) { Remove-Item $Dest -Force }
    Move-Item $ExtractedPath $Dest

    $currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")
    if ($currentPath -notlike "*$InstallDir*") {
        try {
            [Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallDir", "Machine")
            $env:PATH = "$env:PATH;$InstallDir"
            Write-Host "==> PATH 등록 완료"
        } catch {
            Write-Warning "PATH 자동 등록 실패. 수동으로 추가하세요: $InstallDir"
        }
    }

    Write-Host ""
    Write-Host "  OK  miraeboy-agent $Version installed -> $Dest" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Quick start:"
    Write-Host "    miraeboy-agent.exe --server http://miraeboy.example.com:9300 --agent-key YOUR_KEY"

} finally {
    Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
