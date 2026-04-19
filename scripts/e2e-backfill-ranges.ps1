# E2E: backfill trade_klines (1m) через control-plane -> NATS -> market-data-ingestor.
# Smoke: статус job=completed, backfill_stats.total_rows в допустимых границах (толеранс к «дырам» биржи), min_ts/max_ts.
# Опционально -DeepValidation: после completed вызывает MDI POST /api/v1/jobs/validate-dataset (нужен -Register и dataset_id в job.result).
#
# Prerequisites: Docker (Postgres, NATS, MinIO), control-plane API + worker, MDI worker, сеть к api.binance.com.
# Ожидаемые минуты для окон совпадают с services/market-data-ingestor/internal/e2eexpect/range_test.go
#
# Usage (из корня репозитория Algorhythm, не из services/market-data-ingestor):
#   cd D:\Projects\Personal\Algorhythm
#   .\scripts\e2e-backfill-ranges.ps1
#   .\scripts\e2e-backfill-ranges.ps1 -ControlPlaneUrl http://127.0.0.1:8080 -Skip6mo
#   .\scripts\e2e-backfill-ranges.ps1 -Register   # регистрация dataset в CP (дольше, засоряет БД)

param(
    [string]$ControlPlaneUrl = "http://localhost:8080",
    [string]$MarketDataIngestorUrl = "http://localhost:8081",
    [string]$Symbol = "BTCUSDT",
    [int]$PollMs = 500,
    # Таймауты на сценарий (сек): длинные окна тянут много страниц Binance.
    [int]$TimeoutWeekSec = 600,
    [int]$TimeoutMonthSec = 900,
    [int]$Timeout3moSec = 1800,
    [int]$Timeout6moSec = 3600,
    [switch]$Skip6mo,
    [switch]$Register,
    [switch]$DeepValidation
)

$ErrorActionPreference = "Stop"
$base = $ControlPlaneUrl.TrimEnd("/")

function Fail([string]$msg) {
    Write-Host "FAIL: $msg" -ForegroundColor Red
    exit 1
}

function MinuteSpan([DateTimeOffset]$from, [DateTimeOffset]$to) {
    if ($to -le $from) { return 0 }
    return [math]::Floor(($to - $from).TotalMinutes)
}

function TradeKlines1mRowBounds([int]$mins) {
    if ($mins -le 0) { return @(0, 0) }
    $slackLow = [math]::Max(50, [math]::Floor($mins * 0.005))
    $minR = $mins - $slackLow
    if ($minR -lt 1) { $minR = 1 }
    $maxR = $mins + 50
    return @($minR, $maxR)
}

# Invoke-RestMethod иногда ломает вложенный job.Result; тянем сырой JSON.
function Get-JobJson([string]$JobId) {
    $u = "$base/api/v1/jobs/$JobId"
    $r = Invoke-WebRequest -Uri $u -UseBasicParsing -TimeoutSec 60
    return ($r.Content | ConvertFrom-Json)
}

function Get-JobResultObject($job) {
    $res = $job.Result
    if (-not $res) { $res = $job.result }
    if ($null -eq $res) { return $null }
    if ($res -is [string]) { return ($res | ConvertFrom-Json) }
    return $res
}

function Get-BackfillStats($resultObj) {
    if ($null -eq $resultObj) { return $null }
    return $resultObj.backfill_stats
}

# Те же окна, что в e2eexpect.TestTradeKlines1mRowBounds_scenariosMatchScript
$Scenarios = @(
    @{ Name = "week"; From = "2024-11-24T00:00:00Z"; To = "2024-12-01T00:00:00Z"; TimeoutSec = $TimeoutWeekSec }
    @{ Name = "month"; From = "2024-11-01T00:00:00Z"; To = "2024-12-01T00:00:00Z"; TimeoutSec = $TimeoutMonthSec }
    @{ Name = "3mo"; From = "2024-09-01T00:00:00Z"; To = "2024-12-01T00:00:00Z"; TimeoutSec = $Timeout3moSec }
    @{ Name = "6mo"; From = "2024-06-01T00:00:00Z"; To = "2024-12-01T00:00:00Z"; TimeoutSec = $Timeout6moSec }
)

if ($Skip6mo) {
    $Scenarios = $Scenarios | Where-Object { $_.Name -ne "6mo" }
}

Write-Host "=== e2e-backfill-ranges: $base symbol=$Symbol register=$Register deep=$($DeepValidation.IsPresent) ===" -ForegroundColor Cyan

try {
    $ready = Invoke-RestMethod -Uri "$base/readyz" -TimeoutSec 10
    if ($ready -ne "ok") { Fail "readyz returned: $ready" }
} catch {
    Fail "control-plane not reachable at $base/readyz - start stack (see README / start-desktop-stack.ps1)"
}
Write-Host "readyz: ok"

# Убедиться, что API собран с POST .../dataset-ready-sync (иначе MDI не сможет записать backfill_stats).
$prefUrl = "$base/api/v1/jobs/00000000-0000-0000-0000-000000000000/dataset-ready-sync"
$prefBody = '{"job_id":"00000000-0000-0000-0000-000000000000","dataset_kind":"trade_klines","symbol":"X"}'
try {
    Invoke-WebRequest -Method Post -Uri $prefUrl -Body $prefBody -ContentType "application/json; charset=utf-8" -UseBasicParsing -TimeoutSec 15 | Out-Null
} catch {
    $code = $null
    try { $code = [int]$_.Exception.Response.StatusCode } catch {}
    if ($code -eq 404) {
        Fail 'control-plane returns 404 on POST dataset-ready-sync - rebuild control-plane, restart cmd/api'
    }
    Write-Host "preflight dataset-ready-sync: $($_.Exception.Message) - continuing if not 404" -ForegroundColor Yellow
}

# Быстрая проверка, что ожидания в Go-тестах согласованы (опционально)
$mdiRoot = Join-Path (Split-Path $PSScriptRoot -Parent) "services\market-data-ingestor"
if (Test-Path $mdiRoot) {
    Push-Location $mdiRoot
    try {
        go test -count=1 "./internal/e2eexpect/..." 2>&1 | ForEach-Object { Write-Host $_ }
        if ($LASTEXITCODE -ne 0) { Fail 'go test ./internal/e2eexpect/... failed - fix bounds or scenarios' }
    } finally {
        Pop-Location
    }
}

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$reg = $Register.IsPresent

foreach ($sc in $Scenarios) {
    $name = $sc.Name
    $fromS = $sc.From
    $toS = $sc.To
    $deadlineSec = $sc.TimeoutSec
    $fromDto = [DateTimeOffset]::Parse($fromS)
    $toDto = [DateTimeOffset]::Parse($toS)
    $mins = MinuteSpan $fromDto $toDto
    $bounds = TradeKlines1mRowBounds $mins
    $minRows = $bounds[0]
    $maxRows = $bounds[1]

    Write-Host "`n--- scenario $name : $fromS .. $toS (span ${mins} min, expect rows in [$minRows, $maxRows]) ---" -ForegroundColor Yellow

    $extId = "e2e_${name}_${ts}"
    $body = @{
        dataset_kind = "trade_klines"
        symbol       = $Symbol
        interval     = "1m"
        from         = $fromS
        to           = $toS
        register     = $reg
        external_id  = $extId
    } | ConvertTo-Json

    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $post = Invoke-WebRequest -Method Post -Uri "$base/api/v1/jobs/backfill/request" -ContentType "application/json" -Body $body -UseBasicParsing -TimeoutSec 60
    $job = $post.Content | ConvertFrom-Json
    $jid = $job.ID
    if (-not $jid) { $jid = $job.id }
    if (-not $jid) { Fail "no job id in response" }
    Write-Host "job_id=$jid"

    $deadline = (Get-Date).AddSeconds($deadlineSec)
    $final = $null
    while ((Get-Date) -lt $deadline) {
        $final = Get-JobJson $jid
        $st = $final.Status
        if (-not $st) { $st = $final.status }
        if ($st -eq "completed" -or $st -eq "failed") { break }
        Start-Sleep -Milliseconds $PollMs
    }
    $sw.Stop()

    if ($null -eq $final) { Fail "no job response" }
    $st = $final.Status; if (-not $st) { $st = $final.status }
    if ($st -ne "completed") {
        Write-Host ($final | ConvertTo-Json -Depth 8)
        Fail "scenario $name : expected completed, got $st"
    }

    # Повторное чтение job: иногда удобно после commit; также сглаживает гонки с merge result.
    $final = Get-JobJson $jid
    $res = Get-JobResultObject $final
    $stats = Get-BackfillStats $res
    if ($null -eq $stats) {
        $deadlinePoll = (Get-Date).AddSeconds(15)
        while ((Get-Date) -lt $deadlinePoll) {
            Start-Sleep -Milliseconds 400
            $final = Get-JobJson $jid
            $res = Get-JobResultObject $final
            $stats = Get-BackfillStats $res
            if ($null -ne $stats) { break }
        }
    }

    if ($null -eq $res) { Fail "scenario $name : empty result" }
    if ($null -eq $stats) {
        Write-Host "DEBUG job.result:" ($res | ConvertTo-Json -Depth 10)
        Fail ('scenario {0}: no backfill_stats - dataset-ready-sync or md.dataset.ready - see README E2E backfill' -f $name)
    }

    $total = [int]$stats.total_rows
    $pcount = [int]$stats.partitions_count
    $minTs = $stats.min_ts
    $maxTs = $stats.max_ts
    $internalVal = $stats.internal_validated
    if ($null -eq $internalVal) { $internalVal = $stats.InternalValidated }

    Write-Host ("rows={0} partitions={1} min_ts={2} max_ts={3} internal_validated={4} wall_sec={5:n1}" -f $total, $pcount, $minTs, $maxTs, $internalVal, $sw.Elapsed.TotalSeconds) -ForegroundColor Green

    if ($total -lt $minRows -or $total -gt $maxRows) {
        Fail "scenario $name : total_rows $total outside [$minRows, $maxRows] (smoke tolerance for exchange gaps)"
    }
    if ($internalVal -ne $true) {
        Write-Host "WARN: internal_validated is not true (internal month checks)" -ForegroundColor Yellow
    }

    if ($DeepValidation.IsPresent) {
        if (-not $Register) {
            Fail "scenario ${name}: -DeepValidation requires -Register (dataset_id on job result)"
        }
        $mdb = $MarketDataIngestorUrl.TrimEnd("/")
        $dsId = $res.dataset_id
        if (-not $dsId) { $dsId = $res.DatasetID }
        if (-not $dsId) { Fail "scenario ${name}: deep gate needs dataset_id in job result" }
        $vBody = @{ dataset_id = $dsId } | ConvertTo-Json
        try {
            $vRaw = Invoke-WebRequest -Method Post -Uri "$mdb/api/v1/jobs/validate-dataset" -Body $vBody -ContentType "application/json; charset=utf-8" -UseBasicParsing -TimeoutSec 600
            $vr = $vRaw.Content | ConvertFrom-Json
        } catch {
            Fail "scenario ${name}: deep validation request failed: $($_.Exception.Message)"
        }
        $ok = $vr.valid
        if (-not $ok) { $ok = $vr.Valid }
        if ($ok -ne $true) {
            Write-Host ($vr | ConvertTo-Json -Depth 8)
            Fail "scenario ${name}: deep validation valid=false"
        }
        Write-Host "deep validation: OK (dataset_id=$dsId)" -ForegroundColor Green
    }

    $minDt = [DateTimeOffset]::Parse($minTs.ToString())
    $maxDt = [DateTimeOffset]::Parse($maxTs.ToString())
    if ($minDt -lt $fromDto.AddMinutes(-2)) {
        Fail "scenario $name : min_ts too far before from"
    }
    if ($maxDt -gt $toDto.AddMinutes(2)) {
        Fail "scenario $name : max_ts after to"
    }
}

Write-Host "`n=== e2e-backfill-ranges: PASS ===" -ForegroundColor Green
exit 0
