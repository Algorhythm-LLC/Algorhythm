# Smoke E2E: control-plane API + experiment run -> backtest-engine -> ClickHouse
# Prerequisites: Docker (Postgres, NATS, ClickHouse), control-plane API + workers + backtest-engine running.
# Usage: .\scripts\smoke-e2e.ps1
#        .\scripts\smoke-e2e.ps1 -ControlPlaneUrl http://127.0.0.1:8080

param(
    [string]$ControlPlaneUrl = "http://localhost:8080",
    [int]$TimeoutSec = 45,
    [string]$ClickHouseContainer = "algorhythm-clickhouse"
)

$ErrorActionPreference = "Stop"
$base = $ControlPlaneUrl.TrimEnd("/")

function Fail([string]$msg) {
    Write-Host "FAIL: $msg" -ForegroundColor Red
    exit 1
}

Write-Host "=== smoke-e2e: $base ===" -ForegroundColor Cyan

try {
    $ready = Invoke-RestMethod -Uri "$base/readyz" -TimeoutSec 5
    if ($ready -ne "ok") { Fail "readyz returned: $ready" }
} catch {
    Fail "control-plane not reachable at $base/readyz — start API and check CP_HTTP_PORT"
}
Write-Host "readyz: ok"

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$fsCode = "smoke_fs_$ts"
$stCode = "smoke_st_$ts"

$fs = Invoke-RestMethod -Method Post -Uri "$base/api/v1/feature-sets" -ContentType "application/json" `
    -Body (@{ code = $fsCode; description = "smoke-e2e" } | ConvertTo-Json)
$fsv = Invoke-RestMethod -Method Post -Uri "$base/api/v1/feature-set-versions" -ContentType "application/json" `
    -Body (@{ feature_set_id = $fs.id; version = 1; schema_json = @{ v = 1 } } | ConvertTo-Json -Compress)
$null = Invoke-RestMethod -Method Post -Uri "$base/api/v1/strategy-templates" -ContentType "application/json" `
    -Body (@{ code = $stCode; name = "Smoke"; description = "" } | ConvertTo-Json)
$sv = Invoke-RestMethod -Method Post -Uri "$base/api/v1/strategy-versions" -ContentType "application/json" `
    -Body (@{ strategy_template_code = $stCode; version = 1; dsl_json = @{ schema_version = 1 } } | ConvertTo-Json -Compress)
$batch = Invoke-RestMethod -Method Post -Uri "$base/api/v1/experiment-batches" -ContentType "application/json" `
    -Body (@{ name = "smoke_batch_$ts"; feature_set_version_id = $fsv.id; symbol_universe_json = @("BTCUSDT") } | ConvertTo-Json -Compress)
$run = Invoke-RestMethod -Method Post -Uri "$base/api/v1/experiment-runs/request" -ContentType "application/json" `
    -Body (@{ experiment_batch_id = $batch.id; strategy_version_id = $sv.id; symbol = "BTCUSDT"; parameters = @{ smoke = $true } } | ConvertTo-Json -Compress)

$rid = $run.id
Write-Host "experiment_run: $rid initial_status=$($run.status)"

$deadline = (Get-Date).AddSeconds($TimeoutSec)
$final = $null
while ((Get-Date) -lt $deadline) {
    $final = Invoke-RestMethod -Uri "$base/api/v1/experiment-runs/$rid"
    if ($final.status -eq "completed" -or $final.status -eq "failed") { break }
    Start-Sleep -Milliseconds 400
}

if ($null -eq $final) { Fail "no response for experiment run" }
if ($final.status -ne "completed") {
    Write-Host ($final | ConvertTo-Json -Depth 6)
    Fail "expected status completed, got $($final.status)"
}
Write-Host "experiment_run: completed" -ForegroundColor Green

try {
    $exists = docker ps -q -f "name=$ClickHouseContainer" 2>$null
    if ($exists) {
        $q = "SELECT count() FROM default.backtest_run_summaries WHERE run_id = '$rid' FORMAT TabSeparated"
        $cnt = docker exec $ClickHouseContainer clickhouse-client --user default --password clickhouse -q $q 2>$null
        if ($cnt -eq "1") {
            Write-Host "clickhouse: row present for run_id (ok)" -ForegroundColor Green
        } else {
            Write-Host "clickhouse: expected 1 row, got '$cnt' (optional check)" -ForegroundColor Yellow
        }
    } else {
        Write-Host "clickhouse: container '$ClickHouseContainer' not running — skipped" -ForegroundColor Yellow
    }
} catch {
    Write-Host "clickhouse: check skipped ($($_.Exception.Message))" -ForegroundColor Yellow
}

Write-Host "=== smoke-e2e: PASS ===" -ForegroundColor Green
exit 0
