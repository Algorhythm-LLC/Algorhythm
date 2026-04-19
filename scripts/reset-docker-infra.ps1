<#
.SYNOPSIS
  Сброс данных только инфраструктуры Docker (ops/full-stack): именованные тома Postgres, MinIO, ClickHouse, NATS, Qdrant.

.DESCRIPTION
  Выполняет `docker compose down -v` в каталоге ops/full-stack - удаляются именованные volumes, состояние внутри контейнеров обнуляется.
  Локальные файлы на ПК (TEMP, data control-desktop, настройки) не трогаются.

  После сброса поднимите стенд снова: .\scripts\up-full-stack.ps1 или .\scripts\run.ps1

.EXAMPLE
  .\scripts\reset-docker-infra.ps1 -Force
#>
param(
    [switch]$Force
)

$ErrorActionPreference = 'Stop'

if (-not $Force) {
    Write-Error "Уничтожает именованные Docker-тома full-stack. Укажите -Force."
    exit 1
}

$OpsDir = Join-Path $PSScriptRoot "..\ops\full-stack"
if (-not (Test-Path (Join-Path $OpsDir "docker-compose.yml"))) {
    Write-Error "Не найден ops/full-stack/docker-compose.yml"
    exit 1
}

Push-Location $OpsDir
try {
    docker compose down -v
    Write-Host "Готово: docker compose down -v (тома удалены)." -ForegroundColor Green
}
finally {
    Pop-Location
}
