<#
.SYNOPSIS
  Одна точка входа: поднять инфраструктуру Docker, control-plane (API + worker),
  market-data-ingestor worker, дождаться готовности API, собрать control-desktop и открыть GUI.

.DESCRIPTION
  Порядок:
  1) docker compose (ops/full-stack)
  2) ожидание Postgres / NATS / MinIO
  3) фон: control-plane API, control-plane worker, MDI worker (логи в $env:TEMP\algorhythm-dev\logs)
  4) ожидание GET /readyz
  5) npm run build + wails build для services/control-desktop
  6) запуск control-desktop.exe

  Повторный запуск: по умолчанию останавливаются процессы из предыдущего прогона (файл stack_pids.json)
  и процесс, слушающий порт control-plane (CP_HTTP_PORT), затем всё поднимается заново.

.PARAMETER SkipDocker
  Не трогать docker compose (инфра уже поднята).

.PARAMETER SkipBuild
  Не собирать фронт/Wails (использовать уже собранный build\bin\control-desktop.exe).

.PARAMETER NoGui
  Не запускать exe в конце (только поднять стенд и сборку).

.PARAMETER ReadyTimeoutSec
  Таймаут ожидания /readyz (по умолчанию 180).

.PARAMETER NoStop
  Не останавливать предыдущие PID и не освобождать порт — при занятом порте скрипт завершится с ошибкой (старое поведение).
#>
param(
    [switch]$SkipDocker,
    [switch]$SkipBuild,
    [switch]$NoGui,
    [switch]$NoStop,
    [int]$ReadyTimeoutSec = 180
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$OpsDir = Join-Path $RepoRoot "ops\full-stack"
$CpDir = Join-Path $RepoRoot "services\control-plane"
$MdiDir = Join-Path $RepoRoot "services\market-data-ingestor"
$DesktopDir = Join-Path $RepoRoot "services\control-desktop"
$LogRoot = Join-Path $env:TEMP "algorhythm-dev"
$LogDir = Join-Path $LogRoot "logs"
# Имя без префикса Pid*: иначе $PidFile читается как $Pid + "File" (встроенная переменная).
$StackPidFile = Join-Path $LogRoot "stack_pids.json"

function Write-Step([string]$msg) {
    Write-Host "`n=== $msg ===" -ForegroundColor Cyan
}

function Import-DotEnv([string]$Path) {
    if (-not (Test-Path $Path)) { return }
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $i = $line.IndexOf("=")
        if ($i -lt 1) { return }
        $k = $line.Substring(0, $i).Trim()
        $v = $line.Substring($i + 1).Trim()
        if ($v.StartsWith('"') -and $v.EndsWith('"')) { $v = $v.Substring(1, $v.Length - 2) }
        [Environment]::SetEnvironmentVariable($k, $v, "Process")
    }
}

function Wait-DockerReady {
    Write-Step "Ожидание контейнеров (postgres, nats, minio)"
    $deadline = (Get-Date).AddSeconds(120)
    while ((Get-Date) -lt $deadline) {
        try {
            $pg = docker exec algorhythm-postgres pg_isready -U algorhythm 2>$null
            $okPg = ($LASTEXITCODE -eq 0)
        } catch { $okPg = $false }
        try {
            $null = Invoke-WebRequest -Uri "http://localhost:8222/healthz" -TimeoutSec 2 -UseBasicParsing
            $okNats = $true
        } catch { $okNats = $false }
        try {
            $null = Invoke-WebRequest -Uri "http://localhost:9000/minio/health/live" -TimeoutSec 2 -UseBasicParsing
            $okMinio = $true
        } catch { $okMinio = $false }
        if ($okPg -and $okNats -and $okMinio) {
            Write-Host "Инфраструктура Docker отвечает."
            return
        }
        Write-Host "  ждём... pg=$okPg nats=$okNats minio=$okMinio"
        Start-Sleep -Seconds 2
    }
    throw "Таймаут ожидания Docker. Проверьте: docker ps, порты 5432, 4222, 9000."
}

function Wait-Readyz {
    param([string]$Url, [int]$TimeoutSec)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $base = $Url.TrimEnd("/")
    while ((Get-Date) -lt $deadline) {
        try {
            $resp = Invoke-WebRequest -Uri "$base/readyz" -TimeoutSec 3 -UseBasicParsing
            if ($resp.StatusCode -eq 200 -and ($resp.Content.Trim() -eq "ok")) {
                Write-Host "control-plane readyz: ok" -ForegroundColor Green
                return
            }
        } catch {
            Write-Host "  ожидание API $base/readyz ..."
        }
        Start-Sleep -Seconds 2
    }
    throw "Таймаут: $base/readyz не ответил за $TimeoutSec с. См. логи в $LogDir"
}

function Test-TcpListen([int]$Port) {
    $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    return $null -ne $c
}

function Stop-ListenersOnPort([int]$Port) {
    $seen = @{}
    $conns = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue)
    foreach ($c in $conns) {
        $procId = $c.OwningProcess
        if ($procId -le 0 -or $seen.ContainsKey($procId)) { continue }
        $seen[$procId] = $true
        $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
        $name = if ($proc) { $proc.ProcessName } else { "?" }
        Write-Host "  Освобождаю порт ${Port}: останавливаю PID $procId ($name)" -ForegroundColor Yellow
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }
}

function Stop-StackFromPidFile([string]$Path) {
    if (-not (Test-Path $Path)) { return }
    try {
        $j = Get-Content $Path -Raw -Encoding UTF8 | ConvertFrom-Json
    } catch {
        return
    }
    foreach ($prop in @("control_plane_api", "control_plane_worker", "mdi_worker")) {
        $id = $j.$prop
        if ($null -eq $id) { continue }
        try {
            $pidInt = [int]$id
        } catch {
            continue
        }
        if ($pidInt -le 0) { continue }
        $proc = Get-Process -Id $pidInt -ErrorAction SilentlyContinue
        if (-not $proc) { continue }
        Write-Host "  Останавливаю предыдущий стек: $prop (PID $pidInt, $($proc.ProcessName))" -ForegroundColor Yellow
        Stop-Process -Id $pidInt -Force -ErrorAction SilentlyContinue
    }
}

function Wait-PortFree([int]$Port, [int]$TimeoutSec) {
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (-not (Test-TcpListen -Port $Port)) {
            return
        }
        Start-Sleep -Milliseconds 400
    }
    throw "Порт $Port всё ещё занят после остановки процессов. Закройте приложение вручную или используйте -NoStop и задайте другой CP_HTTP_PORT."
}

# --- main ---
Write-Step "Корень репозитория: $RepoRoot"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

if (-not $SkipDocker) {
    Write-Step "Docker Compose (ops/full-stack)"
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker не найден в PATH."
    }
    Push-Location $OpsDir
    if (-not (Test-Path ".env")) { Copy-Item ".env.example" ".env" }
    docker compose up -d
    Pop-Location
    Wait-DockerReady
} else {
    Write-Host "SkipDocker: пропуск docker compose" -ForegroundColor Yellow
}

# .env для сервисов
if (-not (Test-Path (Join-Path $CpDir ".env"))) {
    Copy-Item (Join-Path $CpDir ".env.example") (Join-Path $CpDir ".env")
}
if (-not (Test-Path (Join-Path $MdiDir ".env"))) {
    Copy-Item (Join-Path $MdiDir ".env.example") (Join-Path $MdiDir ".env")
}

Write-Step "Запуск Go-процессов в фоне (логи: $LogDir)"
Import-DotEnv (Join-Path $CpDir ".env")

$goExe = (Get-Command go).Source
$env:CP_POSTGRES_DSN = if ($env:CP_POSTGRES_DSN) { $env:CP_POSTGRES_DSN } else { "postgres://algorhythm:algorhythm@localhost:5432/control_plane?sslmode=disable" }
$env:CP_NATS_URL = if ($env:CP_NATS_URL) { $env:CP_NATS_URL } else { "nats://localhost:4222" }
$env:CP_HTTP_PORT = if ($env:CP_HTTP_PORT) { $env:CP_HTTP_PORT } else { "8080" }
$cpPort = [int]$env:CP_HTTP_PORT
$ControlPlaneUrl = "http://localhost:$cpPort"

if (-not $NoStop) {
    Write-Host "Перезапуск стека: останавливаю предыдущие процессы (если были)…" -ForegroundColor DarkGray
    Stop-StackFromPidFile -Path $StackPidFile
    if (Test-TcpListen -Port $cpPort) {
        Stop-ListenersOnPort -Port $cpPort
        Wait-PortFree -Port $cpPort -TimeoutSec 20
    }
} elseif (Test-TcpListen -Port $cpPort) {
    throw "Порт $cpPort занят. Уберите -NoStop для автоматической остановки или освободите порт вручную."
}

$pApi = Start-Process -FilePath $goExe -ArgumentList @("run", "./cmd/api") -WorkingDirectory $CpDir `
    -RedirectStandardOutput (Join-Path $LogDir "control-plane-api.log") `
    -RedirectStandardError (Join-Path $LogDir "control-plane-api.err") `
    -PassThru -WindowStyle Hidden

Start-Sleep -Milliseconds 500

$pWrk = Start-Process -FilePath $goExe -ArgumentList @("run", "./cmd/worker") -WorkingDirectory $CpDir `
    -RedirectStandardOutput (Join-Path $LogDir "control-plane-worker.log") `
    -RedirectStandardError (Join-Path $LogDir "control-plane-worker.err") `
    -PassThru -WindowStyle Hidden

# MDI worker — переменные из .env ingestor (без наследования CP_*)
foreach ($k in [Environment]::GetEnvironmentVariables("Process").Keys) {
    if ($k -like "CP_*") { [Environment]::SetEnvironmentVariable($k, $null, "Process") }
}
Import-DotEnv (Join-Path $MdiDir ".env")
$pMdi = Start-Process -FilePath $goExe -ArgumentList @("run", "./cmd/worker") -WorkingDirectory $MdiDir `
    -RedirectStandardOutput (Join-Path $LogDir "market-data-ingestor-worker.log") `
    -RedirectStandardError (Join-Path $LogDir "market-data-ingestor-worker.err") `
    -PassThru -WindowStyle Hidden

Write-Host "PID control-plane API:    $($pApi.Id)"
Write-Host "PID control-plane worker: $($pWrk.Id)"
Write-Host "PID MDI worker:           $($pMdi.Id)"

$pidsOut = [ordered]@{
    control_plane_api    = $pApi.Id
    control_plane_worker = $pWrk.Id
    mdi_worker           = $pMdi.Id
    updated_at           = (Get-Date).ToString("o")
}
$pidsOut | ConvertTo-Json | Set-Content -Path $StackPidFile -Encoding UTF8
Write-Host "PID saved: $StackPidFile" -ForegroundColor DarkGray

Write-Step "Ожидание control-plane ($ControlPlaneUrl)"
Wait-Readyz -Url $ControlPlaneUrl -TimeoutSec $ReadyTimeoutSec

if (-not $SkipBuild) {
    Write-Step "Сборка frontend (Vite)"
    Push-Location (Join-Path $DesktopDir "frontend")
    npm run build
    Pop-Location
    Write-Step "Сборка Wails (control-desktop)"
    $wails = Get-Command wails -ErrorAction SilentlyContinue
    if (-not $wails) {
        Write-Host "wails не в PATH — используем go run для CLI Wails" -ForegroundColor Yellow
        & go run github.com/wailsapp/wails/v2/cmd/wails@latest build
    } else {
        Push-Location $DesktopDir
        wails build
        Pop-Location
    }
} else {
    Write-Host "SkipBuild: пропуск сборки" -ForegroundColor Yellow
}

$exe = Join-Path $DesktopDir "build\bin\control-desktop.exe"
if (-not (Test-Path $exe)) {
    throw "Не найден $exe — уберите -SkipBuild или соберите вручную (wails build)."
}

if (-not $NoGui) {
    Write-Step "Запуск GUI"
    Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe)
    Write-Host "Запущено: $exe" -ForegroundColor Green
} else {
    Write-Host "NoGui: exe не запускался. Запуск вручную: $exe" -ForegroundColor Yellow
}

Write-Step "Done"
Write-Host "Логи фоновых процессов: $LogDir"
Write-Host "Next run stops the same stack (see stack_pids.json): $StackPidFile" -ForegroundColor DarkGray
