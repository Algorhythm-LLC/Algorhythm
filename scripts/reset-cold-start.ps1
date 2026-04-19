<#
.SYNOPSIS
  Cold reset: Docker full-stack volumes + local dev artifacts on this PC.

.DESCRIPTION
  1) Stop stack from stack_pids.json, free CP_HTTP_PORT, stop control-desktop.exe (same as start-desktop-stack default pre-start cleanup)
  2) docker compose down -v (Postgres, MinIO, ClickHouse, NATS, Qdrant volumes)
  3) %TEMP%\algorhythm-dev (stack logs/PIDs from start-desktop-stack; retried if locked)
  4) services\control-desktop\build\bin\data (typical Wails build data dir)
  5) By default: %AppData%\Roaming\algorhythm (control-desktop.json)

  Custom DataDir from settings is not detected - delete manually if needed.

.PARAMETER KeepControlDesktopSettings
  Do not remove %AppData%\Roaming\algorhythm.

.EXAMPLE
  .\scripts\reset-cold-start.ps1 -Force

.EXAMPLE
  .\scripts\reset-cold-start.ps1 -Force -KeepControlDesktopSettings
#>
param(
    [switch]$Force,
    [switch]$KeepControlDesktopSettings
)

$ErrorActionPreference = 'Stop'

if (-not $Force) {
    Write-Error "Destructive: pass -Force."
    exit 1
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
. (Join-Path $PSScriptRoot "stop-algorhythm-dev-stack.ps1")
Invoke-AlgorhythmDevStackCleanup -RepoRoot $RepoRoot

$OpsDir = Join-Path $RepoRoot "ops\full-stack"

if (-not (Test-Path (Join-Path $OpsDir "docker-compose.yml"))) {
    Write-Error "Missing ops/full-stack/docker-compose.yml"
    exit 1
}

Write-Host "`n=== Docker: compose down -v ===" -ForegroundColor Cyan
Push-Location $OpsDir
try {
    docker compose down -v
}
finally {
    Pop-Location
}
Write-Host "Docker volumes removed." -ForegroundColor Green

$DevTemp = Join-Path $env:TEMP "algorhythm-dev"
if (Test-Path $DevTemp) {
    Write-Host "`n=== TEMP: $DevTemp ===" -ForegroundColor Cyan
    $devTempGone = $false
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        if ($attempt -gt 1) {
            Write-Host "TEMP still locked; stop stack + go (temp) again (attempt $attempt/3)..." -ForegroundColor Yellow
            Invoke-AlgorhythmDevStackCleanup -RepoRoot $RepoRoot -Quiet
            Start-Sleep -Seconds 2
        }
        if (Remove-AlgorhythmDevTempTree -Path $DevTemp) {
            $devTempGone = $true
            Write-Host "Removed." -ForegroundColor Green
            break
        }
    }
    if (-not $devTempGone) {
        Write-Host "Diagnostics:" -ForegroundColor Yellow
        Show-AlgorhythmDevLockSuspects -Hint "lock on $DevTemp" -Path $DevTemp
        Write-Warning "Could not remove $DevTemp after retries. Stop the listed PIDs manually (e.g. Stop-Process -Id <PID> -Force) or sign out, then run again."
    }
}

$DesktopData = Join-Path $RepoRoot "services\control-desktop\build\bin\data"
if (Test-Path $DesktopData) {
    Write-Host "`n=== control-desktop build data: $DesktopData ===" -ForegroundColor Cyan
    try {
        Remove-Item -Recurse -Force $DesktopData -ErrorAction Stop
        Write-Host "Removed." -ForegroundColor Green
    } catch {
        Write-Warning "Could not remove $DesktopData - close control-desktop.exe and retry."
    }
}

if (-not $KeepControlDesktopSettings) {
    $RoamingAlgo = Join-Path ([Environment]::GetFolderPath('ApplicationData')) "algorhythm"
    if (Test-Path $RoamingAlgo) {
        Write-Host "`n=== Roaming settings: $RoamingAlgo ===" -ForegroundColor Cyan
        try {
            Remove-Item -Recurse -Force $RoamingAlgo -ErrorAction Stop
            Write-Host "Removed (GUI settings reset)." -ForegroundColor Green
        } catch {
            Write-Warning "Could not remove $RoamingAlgo - close the app and retry."
        }
    }
} else {
    Write-Host "`nSkipping Roaming algorhythm (-KeepControlDesktopSettings)." -ForegroundColor Yellow
}

Write-Host "`nDone. Start stack again:" -ForegroundColor Green
Write-Host "  .\scripts\run.ps1" -ForegroundColor Gray
Write-Host "  .\scripts\up-full-stack.ps1" -ForegroundColor Gray
