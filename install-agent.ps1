# miraeboy-agent Windows installer + 빌드 환경 자동 설정
#
# 사용법:
#   irm https://raw.githubusercontent.com/corbie79/miraeboy/main/install-agent.ps1 | iex
#   iex "& { $(irm .../install-agent.ps1) } -SkipDevtools"
#   iex "& { $(irm .../install-agent.ps1) } -Version v1.2.0"
#
# 옵션:
#   -SkipDevtools     빌드 도구 설치 건너뜀
#   -Version V        에이전트 버전 지정 (기본: latest)
#   -InstallDir DIR   설치 경로 (기본: C:\Program Files\miraeboy)
#
[CmdletBinding()]
param(
    [switch]$SkipDevtools,
    [string]$Version    = $env:MIRAEBOY_VERSION,
    [string]$InstallDir = $env:MIRAEBOY_INSTALL_DIR,
    [string]$BaseUrl    = $env:MIRAEBOY_BASE_URL
)

$ErrorActionPreference = 'Stop'
$ProgressPreference    = 'SilentlyContinue'  # Invoke-WebRequest 속도 향상

$Repo   = "corbie79/miraeboy"
$Binary = "miraeboy-agent"
if (-not $BaseUrl) { $BaseUrl = "https://github.com/$Repo" }

# ── 색상 출력 헬퍼 ─────────────────────────────────────────────────────────────

function Step($msg)    { Write-Host "`n━━━ $msg ━━━" -ForegroundColor Cyan }
function Info($msg)    { Write-Host "==>  $msg" -ForegroundColor Cyan }
function Success($msg) { Write-Host " ✓   $msg" -ForegroundColor Green }
function Warn($msg)    { Write-Host "WARN $msg" -ForegroundColor Yellow }

# ── 관리자 권한 확인 ────────────────────────────────────────────────────────────

function Test-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    $p  = [Security.Principal.WindowsPrincipal]$id
    return $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# ── Scoop 설치 ─────────────────────────────────────────────────────────────────

function Install-Scoop {
    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        Info "Scoop 이미 설치됨. 업데이트 중..."
        scoop update
        return
    }
    Step "Scoop 설치"
    Info "Scoop 패키지 매니저 설치 중..."
    Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
    Invoke-RestMethod get.scoop.sh | Invoke-Expression
    # PATH 즉시 반영
    $env:PATH = "$env:USERPROFILE\scoop\shims;$env:PATH"
    Success "Scoop 설치 완료"
}

# ── Visual Studio Build Tools ─────────────────────────────────────────────────

function Install-VsBuildTools {
    # 이미 설치됐는지 확인 (cl.exe 존재 여부)
    $vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
    if (Test-Path $vswhere) {
        $vsPath = & $vswhere -latest -products * -requires Microsoft.VisualCpp.Tools.HostX64.TargetX64 -property installationPath 2>$null
        if ($vsPath) {
            Success "Visual Studio Build Tools 이미 설치됨: $vsPath"
            return
        }
    }

    Step "Visual Studio Build Tools 설치"
    Info "VS Build Tools 2022 다운로드 중... (시간이 걸릴 수 있습니다)"

    $vsInstallerUrl = "https://aka.ms/vs/17/release/vs_BuildTools.exe"
    $vsInstaller    = "$env:TEMP\vs_BuildTools.exe"

    Invoke-WebRequest -Uri $vsInstallerUrl -OutFile $vsInstaller -UseBasicParsing

    Info "VS Build Tools 설치 중 (백그라운드)..."
    $args = @(
        "--quiet", "--wait", "--norestart",
        "--add", "Microsoft.VisualStudio.Workload.VCTools",
        "--add", "Microsoft.VisualCpp.Tools.HostX64.TargetX64",
        "--add", "Microsoft.VisualCpp.Tools.HostX64.TargetX86",
        "--add", "Microsoft.VisualCpp.Tools.HostX64.TargetARM64",
        "--add", "Microsoft.VisualStudio.Component.Windows11SDK.22621",
        "--add", "Microsoft.VisualStudio.Component.VC.CMake.Project"
    )
    $proc = Start-Process -FilePath $vsInstaller -ArgumentList $args -Wait -PassThru
    if ($proc.ExitCode -eq 0 -or $proc.ExitCode -eq 3010) {
        Success "Visual Studio Build Tools 설치 완료"
    } else {
        Warn "VS Build Tools 설치 종료 코드: $($proc.ExitCode). 수동 확인이 필요할 수 있습니다."
    }
    Remove-Item $vsInstaller -Force -ErrorAction SilentlyContinue
}

# ── Scoop 패키지 설치 ─────────────────────────────────────────────────────────

function Install-ScoopPackages {
    Step "빌드 도구 설치 (Scoop)"

    # extras bucket 추가 (winget 등 추가 패키지용)
    scoop bucket add extras 2>$null
    scoop bucket add versions 2>$null

    $packages = @(
        "git",
        "cmake",
        "ninja",
        "nasm",        # 어셈블러 (openssl 빌드 등)
        "yasm",        # 어셈블러 (ffmpeg 빌드 등)
        "openssl",     # OpenSSL 라이브러리
        "ffmpeg",      # FFmpeg (dev 헤더 포함)
        "python",      # Python 3
        "pkgconfiglite",  # pkg-config
        "llvm",        # Clang/LLVM (Clang-cl 포함)
        "make",        # GNU make
        "wget",
        "curl",
        "7zip"
    )

    Info "패키지 업데이트..."
    scoop update *>$null

    foreach ($pkg in $packages) {
        if (scoop list $pkg 2>$null | Select-String $pkg) {
            scoop update $pkg 2>$null
            Success "$pkg 업데이트 완료"
        } else {
            Info "$pkg 설치 중..."
            scoop install $pkg 2>$null
            Success "$pkg 설치 완료"
        }
    }
}

# ── Go 설치 확인 ──────────────────────────────────────────────────────────────

function Ensure-Go {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        $goVer = (go version).Split(" ")[2]
        Success "Go 이미 설치됨: $goVer"
        return
    }

    Step "Go 설치"
    $goVersion = "1.22.4"
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    $goFile = "go${goVersion}.windows-${arch}.msi"
    $goUrl  = "https://go.dev/dl/$goFile"
    $goMsi  = "$env:TEMP\$goFile"

    Info "Go $goVersion 다운로드 중..."
    Invoke-WebRequest -Uri $goUrl -OutFile $goMsi -UseBasicParsing

    Info "Go 설치 중..."
    Start-Process msiexec -ArgumentList "/i `"$goMsi`" /quiet /norestart" -Wait
    Remove-Item $goMsi -Force -ErrorAction SilentlyContinue

    # PATH 즉시 반영
    $env:PATH = "C:\Program Files\Go\bin;$env:PATH"
    Success "Go $goVersion 설치 완료"
}

# ── 에이전트 바이너리 설치 ────────────────────────────────────────────────────

function Install-AgentBinary {
    Step "miraeboy-agent 설치"

    $Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

    if (-not $Version) {
        Info "최신 버전 조회..."
        try {
            $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
            $script:Version = $rel.tag_name
        } catch {
            Warn "버전 자동 조회 실패. -Version vX.Y.Z 를 직접 지정하세요."
            return
        }
        Info "최신 버전: $script:Version"
    }

    $Archive     = "$Binary-windows-$Arch-$script:Version.zip"
    $DownloadUrl = "$BaseUrl/releases/download/$script:Version/$Archive"
    $ChecksumUrl = "$BaseUrl/releases/download/$script:Version/checksums.txt"

    if (-not $InstallDir) { $InstallDir = "$env:ProgramFiles\miraeboy" }
    if (-not (Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }

    $TmpDir = Join-Path $env:TEMP "miraeboy-agent-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
    New-Item -ItemType Directory -Path $TmpDir | Out-Null

    try {
        Info "다운로드: $Archive"
        $ArchivePath = Join-Path $TmpDir $Archive
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing

        # 체크섬 검증
        try {
            $csPath = Join-Path $TmpDir "checksums.txt"
            Invoke-WebRequest -Uri $ChecksumUrl -OutFile $csPath -UseBasicParsing -ErrorAction Stop
            $expected = (Get-Content $csPath | Where-Object { $_ -match "  $Archive$" }) -replace "  $Archive$","" -replace "\s",""
            $actual   = (Get-FileHash $ArchivePath -Algorithm SHA256).Hash.ToLower()
            if ($expected -and $expected -eq $actual) { Info "체크섬 OK" }
            elseif ($expected) { throw "체크섬 불일치!" }
        } catch [System.Net.WebException] { Warn "checksums.txt 없음, 검증 건너뜀" }

        Info "압축 해제..."
        Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force
        $src = Join-Path $TmpDir "$Binary-windows-$Arch.exe"
        $dst = Join-Path $InstallDir "miraeboy-agent.exe"
        if (Test-Path $dst) { Remove-Item $dst -Force }
        Move-Item $src $dst

        # PATH 등록
        $syspath = [Environment]::GetEnvironmentVariable("PATH","Machine")
        if ($syspath -notlike "*$InstallDir*") {
            try {
                [Environment]::SetEnvironmentVariable("PATH","$syspath;$InstallDir","Machine")
                $env:PATH = "$env:PATH;$InstallDir"
                Info "시스템 PATH에 $InstallDir 추가"
            } catch { Warn "PATH 자동 등록 실패 (관리자 권한 필요). 수동으로 추가: $InstallDir" }
        }
        Success "miraeboy-agent $script:Version 설치 완료 → $dst"
    } finally {
        Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# ── 메인 ──────────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "  miraeboy-agent 설치 스크립트 (Windows)" -ForegroundColor Cyan
Write-Host "  ARCH: $env:PROCESSOR_ARCHITECTURE"
Write-Host ""

if (-not $SkipDevtools) {
    if (-not (Test-Admin)) {
        Warn "관리자 권한이 없습니다. VS Build Tools 설치가 제한될 수 있습니다."
        Warn "관리자 PowerShell에서 실행하면 더 완전한 설치가 가능합니다."
    }

    Install-Scoop
    Install-ScoopPackages
    Install-VsBuildTools
    Ensure-Go
} else {
    Info "-SkipDevtools: 빌드 도구 설치 건너뜀"
}

Install-AgentBinary

Write-Host ""
Write-Host "  Quick start:" -ForegroundColor Green
Write-Host "    miraeboy-agent.exe --server http://miraeboy.example.com:9300 --agent-key YOUR_KEY"
Write-Host ""
Write-Host "  Windows 서비스로 등록하려면:"
Write-Host "    sc.exe create miraeboy-agent binPath= `"$InstallDir\miraeboy-agent.exe --server URL --agent-key KEY`" start= auto"
Write-Host "    sc.exe start miraeboy-agent"
