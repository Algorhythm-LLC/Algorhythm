<#
.SYNOPSIS
  Совместимость: Docker-тома full-stack + TEMP\algorhythm-dev (без данных GUI на диске и без Roaming).

.DESCRIPTION
  Для более узкого или полного сброса используйте:
  - .\scripts\reset-docker-infra.ps1 - только контейнеры/тома Docker;
  - .\scripts\reset-cold-start.ps1 - Docker + локальный ПК (данные сборки, опционально настройки).

.EXAMPLE
  .\scripts\reset-dev-data.ps1 -Force
#>
param(
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

if (-not $Force) {
    Write-Error "Уничтожает тома Docker и TEMP\algorhythm-dev. Укажите -Force."
    exit 1
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
. (Join-Path $PSScriptRoot "stop-algorhythm-dev-stack.ps1")
Invoke-AlgorhythmDevStackCleanup -RepoRoot $RepoRoot

& "$PSScriptRoot\reset-docker-infra.ps1" -Force

$DevTemp = Join-Path $env:TEMP "algorhythm-dev"
if (Test-Path $DevTemp) {
    $devTempGone = $false
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        if ($attempt -gt 1) {
            Write-Host "Повтор после остановки стека ($attempt/3)..." -ForegroundColor Yellow
            Invoke-AlgorhythmDevStackCleanup -RepoRoot $RepoRoot -Quiet
            Start-Sleep -Seconds 2
        }
        if (Remove-AlgorhythmDevTempTree -Path $DevTemp) {
            $devTempGone = $true
            Write-Host "Удалено: $DevTemp" -ForegroundColor Green
            break
        }
    }
    if (-not $devTempGone) {
        Write-Host "Diagnostics:" -ForegroundColor Yellow
        Show-AlgorhythmDevLockSuspects -Hint "lock on $DevTemp" -Path $DevTemp
        Write-Warning "Не удалось удалить $DevTemp после повторов (файл занят)."
    }
}

Write-Host "Готово (Docker + TEMP). Полный холодный старт ПК: .\scripts\reset-cold-start.ps1 -Force"
