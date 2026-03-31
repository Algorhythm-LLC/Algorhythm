<#
  Копирует проверенный снимок Local Archive в каталог imports/ локальной папки данных
  и проверяет manifest.json (schema v1, app_id algorhythm).

  Полная загрузка Parquet в MinIO и строк в PostgreSQL/ClickHouse выполняется отдельными
  средствами стенда; здесь — безопасная подготовка файлов на диске.

  Пример:
    .\scripts\restore-desktop-backup.ps1 -SnapshotPath "D:\data\backups\auto\bt_run_xxx" -TargetDataRoot "D:\Algorhythm\data"
#>
param(
    [Parameter(Mandatory = $true)]
    [string] $SnapshotPath,

    [Parameter(Mandatory = $false)]
    [string] $TargetDataRoot = ""
)

$ErrorActionPreference = "Stop"

$snap = Resolve-Path -LiteralPath $SnapshotPath
$snapPath = $snap.Path
if (-not (Test-Path -LiteralPath (Join-Path $snapPath "manifest.json"))) {
    Write-Error "manifest.json не найден: $snapPath"
}

$manifest = Get-Content -LiteralPath (Join-Path $snapPath "manifest.json") -Raw | ConvertFrom-Json
if ($manifest.app_id -ne "algorhythm") {
    Write-Error "Неверный app_id в манифесте: $($manifest.app_id)"
}
if ([string]$manifest.schema_version -ne "1") {
    Write-Warning "schema_version = $($manifest.schema_version) — ожидалась совместимость с v1."
}

if ([string]::IsNullOrWhiteSpace($TargetDataRoot)) {
    Write-Host "Манифест проверен. Укажите -TargetDataRoot, чтобы скопировать снимок в <data>\imports\."
    Write-Host "Компоненты в манифесте:"
    foreach ($c in $manifest.components) {
        Write-Host "  - $($c.name) -> $($c.path) ($($c.format))"
    }
    exit 0
}

$imports = Join-Path $TargetDataRoot "imports"
$destName = Split-Path -Leaf $snapPath
$dest = Join-Path $imports $destName
New-Item -ItemType Directory -Force -Path $imports | Out-Null
if (Test-Path -LiteralPath $dest) {
    Write-Error "Целевой каталог уже существует: $dest"
}

Copy-Item -LiteralPath $snapPath -Destination $dest -Recurse
Write-Host "Снимок скопирован в: $dest"
Write-Host "Дальше: загрузка object storage / импорт JSON в БД — по процедурам вашего стенда (см. services/control-desktop/BACKUP.md)."
