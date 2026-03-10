# Остановка полного стенда Algorhythm (Windows)
$OpsDir = Join-Path $PSScriptRoot "..\ops\full-stack"
Set-Location $OpsDir
docker compose down
Write-Host "Infrastructure stopped."
