# Запуск полного стенда Algorhythm (Windows)
$OpsDir = Join-Path $PSScriptRoot "..\ops\full-stack"
Set-Location $OpsDir
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
docker compose up -d
Write-Host "Infrastructure started. Run 'docker compose logs -f' in ops/full-stack for logs."
