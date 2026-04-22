# Stage 6.1 — Canonical vertical E2E (semi-automated)
# Mirrors docs/stages/stage-6-1-canonical-e2e.md: template → draft (advanced) → preflight → publish → batch → run → results-api → compare.
#
# Prerequisites:
#   - control-plane API reachable (default http://localhost:8080)
#   - backtest-engine reachable from CP (CP_BACKTEST_ENGINE_URL) so runs complete
#   - results-api reachable (default http://localhost:8082)
#   - PostgreSQL row feature_set_versions.id = UUID you pass in -FeatureSetVersionId
#     (must resolve to a feature set whose contract satisfies the sample DSL columns)
#
# Usage:
#   .\scripts\stage-6-1-canonical-e2e.ps1 -FeatureSetVersionId "<uuid-from-DB>"
#   .\scripts\stage-6-1-canonical-e2e.ps1 -ControlPlaneUrl http://127.0.0.1:8080 -ResultsApiUrl http://127.0.0.1:8082 -FeatureSetVersionId "<uuid>"

param(
    [Parameter(Mandatory = $true)]
    [string]$FeatureSetVersionId,

    [string]$ControlPlaneUrl = "http://localhost:8080",
    [string]$ResultsApiUrl = "http://localhost:8082",
    [string]$TemplateCode = "stage6_1_canonical_v1",
    [string]$Symbol = "BTCUSDT",
    [int]$RunTimeoutSec = 120
)

$ErrorActionPreference = "Stop"
$cp = $ControlPlaneUrl.TrimEnd("/")
$rs = $ResultsApiUrl.TrimEnd("/")

function Fail([string]$msg) {
    Write-Host "FAIL: $msg" -ForegroundColor Red
    exit 1
}

function Cp([string]$method, [string]$path, $body = $null) {
    $uri = "$cp$path"
    if ($null -eq $body) {
        return Invoke-RestMethod -Method $method -Uri $uri
    }
    return Invoke-RestMethod -Method $method -Uri $uri -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 30 -Compress)
}

function Rs([string]$method, [string]$path) {
    $uri = "$rs$path"
    return Invoke-RestMethod -Method $method -Uri $uri
}

function Wait-ExperimentRun([string]$runId) {
    $deadline = (Get-Date).AddSeconds($RunTimeoutSec)
    while ((Get-Date) -lt $deadline) {
        $r = Cp "GET" "/api/v1/experiment-runs/$runId"
        if ($r.status -eq "completed" -or $r.status -eq "failed") { return $r }
        Start-Sleep -Milliseconds 500
    }
    Fail "experiment run $runId did not finish within ${RunTimeoutSec}s"
}

function New-AdvancedEnvelope([hashtable]$dsl) {
    return @{
        mode                   = "advanced"
        draft_model_version    = "stage6.v1"
        feature_set_version_id = $FeatureSetVersionId
        advanced               = @{ dsl_json = $dsl }
    }
}

function DslV1Baseline([string]$code) {
    return @{
        schema_version    = "1.1.0"
        strategy_code     = $code
        instrument_scope  = @{ exchange = "binance"; symbols = @($Symbol) }
        entry               = @{ type = "indicator_condition"; params = @{ left = "ema_20_gt_ema_50"; right = "" } }
        exit                = @{ type = "tp_sl"; params = @{ take_profit_bps = 400; stop_loss_bps = 150 } }
        filters             = @()
        risk                = @{ type = "fixed_fraction"; params = @{ risk_bps = 100 } }
        execution           = @{ fee_bps = 10; slippage_bps = 5; allow_short = $false }
    }
}

function DslV1WithCloseLong([string]$code) {
    $h = DslV1Baseline $code
    $h["close_long"] = @{ type = "indicator_condition"; params = @{ left = "rsi_14_lt_30000"; right = "" } }
    return $h
}

Write-Host "=== stage-6-1-canonical-e2e: CP=$cp RS=$rs template=$TemplateCode ===" -ForegroundColor Cyan

try {
    $ready = Invoke-RestMethod -Uri "$cp/readyz" -TimeoutSec 5
    if ($ready -ne "ok") { Fail "CP readyz returned: $ready" }
} catch { Fail "control-plane not reachable at $cp/readyz" }

try {
    $r0 = Invoke-RestMethod -Uri "$rs/readyz" -TimeoutSec 5
    if ($r0 -ne "ok") { Fail "results-api readyz returned: $r0" }
} catch { Fail "results-api not reachable at $rs/readyz" }

try {
    $null = Cp "GET" "/api/v1/strategy-templates/$TemplateCode"
} catch {
    $null = Cp "POST" "/api/v1/strategy-templates" @{
        code        = $TemplateCode
        name        = "Stage 6.1 canonical"
        description = "Created by scripts/stage-6-1-canonical-e2e.ps1"
    }
}

function Publish-Version([hashtable]$dsl) {
    $draftBody = @{
        strategy_template_code = $TemplateCode
        draft_json             = (New-AdvancedEnvelope $dsl)
        created_by             = "stage-6-1-canonical-e2e"
    }
    $draft = Cp "POST" "/api/v1/strategy-drafts" $draftBody
    $did = $draft.id
    if (-not $did) { Fail "draft create: missing id" }

    $pf = Cp "POST" "/api/v1/strategy-drafts/$did/preflight" @{}
    if (-not $pf.valid) { Fail "preflight invalid: $($pf | ConvertTo-Json -Depth 8)" }
    if (-not $pf.runtime_supported) { Fail "preflight runtime_supported=false: $($pf | ConvertTo-Json -Depth 8)" }

    $pub = Cp "POST" "/api/v1/strategy-drafts/$did/publish" @{ version = 0 }
    $sv = $pub.strategy_version
    if (-not $sv.id) { Fail "publish: missing strategy_version.id" }
    return $sv.id
}

$svA = Publish-Version (DslV1Baseline $TemplateCode)
$svB = Publish-Version (DslV1WithCloseLong $TemplateCode)
Write-Host "strategy_version A=$svA B=$svB" -ForegroundColor Green

$ts = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$batch = Cp "POST" "/api/v1/experiment-batches" @{
    name                     = "stage61_canonical_$ts"
    feature_set_version_id   = $FeatureSetVersionId
    symbol_universe_json     = @($Symbol)
}
$bid = $batch.id

$runA = Cp "POST" "/api/v1/experiment-runs/request" @{
    experiment_batch_id  = $bid
    strategy_version_id  = $svA
    symbol               = $Symbol
    parameters           = @{}
}
$runB = Cp "POST" "/api/v1/experiment-runs/request" @{
    experiment_batch_id  = $bid
    strategy_version_id  = $svB
    symbol               = $Symbol
    parameters           = @{}
}

$fa = Wait-ExperimentRun $runA.id
$fb = Wait-ExperimentRun $runB.id
if ($fa.status -ne "completed") { Fail "run A $($runA.id) status=$($fa.status)" }
if ($fb.status -ne "completed") { Fail "run B $($runB.id) status=$($fb.status)" }
Write-Host "runs completed A=$($runA.id) B=$($runB.id)" -ForegroundColor Green

$sumA = Rs "GET" "/api/v1/runs/$($runA.id)/summary"
$sumB = Rs "GET" "/api/v1/runs/$($runB.id)/summary"
if ($null -eq $sumA) { Fail "results summary A empty" }
if ($null -eq $sumB) { Fail "results summary B empty" }

$cmpRuns = Rs "GET" "/api/v1/compare/runs?left_run_id=$($runA.id)&right_run_id=$($runB.id)"
if ($null -eq $cmpRuns) { Fail "compare runs empty" }

$cmpVer = Rs "GET" "/api/v1/compare/versions?left_version_id=$svA&right_version_id=$svB"
if ($null -eq $cmpVer) { Fail "compare versions empty" }

Write-Host "=== stage-6-1-canonical-e2e: PASS ===" -ForegroundColor Green
exit 0
