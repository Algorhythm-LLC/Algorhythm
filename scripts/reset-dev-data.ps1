# Полный сброс данных dev-стенда Algorhythm (Windows, PowerShell).
# Удаляет именованные Docker-тома full-stack: MinIO, Postgres, ClickHouse, NATS, Qdrant.
# НЕ трогает настройки control-desktop в AppData — только инфраструктура и temp PIDs стека.
#
# Использование:
#   .\scripts\reset-dev-data.ps1 -Force
#
# После сброса: поднять стенд снова (например scripts\up-full-stack.ps1 или start-desktop-stack.ps1).

param(
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

if (-not $Force) {
    Write-Error "Destructive: removes Docker full-stack volumes. Pass -Force."
    exit 1
}

$OpsDir = Join-Path $PSScriptRoot "..\ops\full-stack"
if (-not (Test-Path (Join-Path $OpsDir "docker-compose.yml"))) {
    Write-Error "ops/full-stack/docker-compose.yml not found."
    exit 1
}

Push-Location $OpsDir
try {
    docker compose down -v
    Write-Host "docker compose down -v OK (volumes removed)."
}
finally {
    Pop-Location
}

$DevTemp = Join-Path $env:TEMP "algorhythm-dev"
if (Test-Path $DevTemp) {
    try {
        Remove-Item -Recurse -Force $DevTemp -ErrorAction Stop
        Write-Host "Removed: $DevTemp"
    } catch {
        Write-Warning "Could not remove $DevTemp (file in use). Stop stack/desktop processes and delete manually, or retry."
    }
}

Write-Host "Done. Start the stack again (e.g. scripts\up-full-stack.ps1)."
