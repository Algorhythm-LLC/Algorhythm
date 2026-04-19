# Dot-source only: helpers for start-desktop-stack.ps1 and reset scripts.
# ASCII-only console text: Windows PowerShell 5.1 reads scripts as system ANSI if no BOM.
# . (Join-Path $PSScriptRoot "stop-algorhythm-dev-stack.ps1")

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
        Write-Host "  Free port ${Port}: stopping PID $procId ($name)" -ForegroundColor Yellow
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
        Write-Host "  Stop stack PID: $prop (PID $pidInt, $($proc.ProcessName))" -ForegroundColor Yellow
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
    throw "Port $Port is still in use after stopping listeners. Close the app or change CP_HTTP_PORT."
}

function Get-ControlPlaneHttpPort([string]$RepoRoot) {
    $p = Join-Path $RepoRoot "services\control-plane\.env"
    $port = 8080
    if (-not (Test-Path $p)) { return $port }
    foreach ($line in Get-Content $p -ErrorAction SilentlyContinue) {
        $t = $line.Trim()
        if ($t.Length -eq 0 -or $t.StartsWith("#")) { continue }
        if ($t -match '^\s*CP_HTTP_PORT\s*=\s*(.+)$') {
            $v = $Matches[1].Trim()
            if ($v.StartsWith('"') -and $v.EndsWith('"')) { $v = $v.Substring(1, $v.Length - 2) }
            $parsed = 0
            if ([int]::TryParse($v, [ref]$parsed)) { return $parsed }
            return $port
        }
    }
    return $port
}

function Stop-ProcessesUsingAlgorhythmDevTemp {
    param([switch]$Quiet)
    $marker = "algorhythm-dev"
    $myPid = $PID

    $procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        if ($null -eq $_) { return $false }
        if ($_.ProcessId -eq $myPid) { return $false }
        $cl = $_.CommandLine
        $ex = $_.ExecutablePath
        if (-not [string]::IsNullOrEmpty($cl) -and $cl -like "*${marker}*") { return $true }
        if (-not [string]::IsNullOrEmpty($ex) -and $ex -like "*${marker}*") { return $true }
        return $false
    }

    foreach ($p in $procs) {
        if (-not $Quiet) {
            Write-Host "  Stop process $($p.ProcessId) ($($p.Name)) (algorhythm-dev on cmdline/path)" -ForegroundColor Yellow
        }
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Stop-ProcessesExecutableUnderCpAndMdiServices {
    param(
        [Parameter(Mandatory)][string]$RepoRoot,
        [switch]$Quiet
    )
    $root = [System.IO.Path]::GetFullPath($RepoRoot)
    $p1 = ((Join-Path $root "services\control-plane") + [IO.Path]::DirectorySeparatorChar).ToLowerInvariant()
    $p2 = ((Join-Path $root "services\market-data-ingestor") + [IO.Path]::DirectorySeparatorChar).ToLowerInvariant()
    $myPid = $PID

    $procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        if ($null -eq $_) { return $false }
        if ($_.ProcessId -eq $myPid) { return $false }
        $ex = $_.ExecutablePath
        if ([string]::IsNullOrEmpty($ex)) { return $false }
        try {
            $full = [System.IO.Path]::GetFullPath($ex).ToLowerInvariant()
        } catch {
            return $false
        }
        return ($full.StartsWith($p1) -or $full.StartsWith($p2))
    }

    foreach ($p in $procs) {
        if (-not $Quiet) {
            $bn = [IO.Path]::GetFileName($p.ExecutablePath)
            Write-Host "  Stop service exe under services\ (PID $($p.ProcessId), $bn)" -ForegroundColor Yellow
        }
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Get-GoBuildCachePrefixes {
    $prefixes = New-Object System.Collections.Generic.List[string]
    try {
        $gocache = (& go env GOCACHE 2>$null)
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($gocache)) {
            $prefixes.Add($gocache.Trim().ToLowerInvariant()) | Out-Null
        }
    } catch { }
    if ($env:GOCACHE) {
        $prefixes.Add($env:GOCACHE.ToLowerInvariant()) | Out-Null
    }
    if ($env:LOCALAPPDATA) {
        $prefixes.Add((Join-Path $env:LOCALAPPDATA "go-build").ToLowerInvariant()) | Out-Null
    }
    $prefixes.Add((Join-Path $env:TEMP "go-build").ToLowerInvariant()) | Out-Null
    # Unique, normalized, without trailing separator
    $seen = @{}
    $result = @()
    foreach ($p in $prefixes) {
        if ([string]::IsNullOrWhiteSpace($p)) { continue }
        $norm = $p.TrimEnd('\', '/')
        if ($seen.ContainsKey($norm)) { continue }
        $seen[$norm] = $true
        $result += $norm
    }
    return $result
}

function Stop-GoBuildTempExecutables {
    param([switch]$Quiet)
    $prefixes = Get-GoBuildCachePrefixes
    $myPid = $PID

    $procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        if ($null -eq $_) { return $false }
        if ($_.ProcessId -eq $myPid) { return $false }
        $ex = $_.ExecutablePath
        if ([string]::IsNullOrEmpty($ex)) { return $false }
        try {
            $full = [System.IO.Path]::GetFullPath($ex).ToLowerInvariant()
        } catch {
            return $false
        }
        foreach ($pref in $prefixes) {
            if ($full.StartsWith($pref)) { return $true }
        }
        return $false
    }

    foreach ($p in $procs) {
        if (-not $Quiet) {
            $bn = [IO.Path]::GetFileName($p.ExecutablePath)
            Write-Host "  Stop go-build cache exe (PID $($p.ProcessId), $bn)" -ForegroundColor Yellow
        }
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Stop-GoProcessesForRepo {
    param(
        [Parameter(Mandatory)][string]$RepoRoot,
        [switch]$Quiet
    )
    $repoLo = $RepoRoot.TrimEnd('\', '/').ToLowerInvariant()
    $cpSvcLo = (Join-Path $RepoRoot "services\control-plane").ToLowerInvariant()
    $mdiSvcLo = (Join-Path $RepoRoot "services\market-data-ingestor").ToLowerInvariant()
    $myPid = $PID

    $procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        if ($null -eq $_) { return $false }
        if ($_.ProcessId -eq $myPid) { return $false }
        if ($_.Name -ne "go.exe") { return $false }
        $cl = $_.CommandLine
        if ([string]::IsNullOrEmpty($cl)) { return $false }
        $clLo = $cl.ToLowerInvariant()
        if ($clLo.Contains($repoLo)) { return $true }
        if ($clLo.Contains($cpSvcLo)) { return $true }
        if ($clLo.Contains($mdiSvcLo)) { return $true }
        # Legacy: ./cmd/worker only — match repo folder name in module path / cwd-style hints
        if ($clLo -match 'cmd[\\/]api' -or $clLo -match 'cmd[\\/]worker') {
            if ($clLo.Contains('\algorhythm\') -or $clLo.Contains('/algorhythm/')) { return $true }
        }
        return $false
    }

    foreach ($p in $procs) {
        if (-not $Quiet) {
            Write-Host "  Stop go.exe for this repo (PID $($p.ProcessId))" -ForegroundColor Yellow
        }
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-AlgorhythmDevStackCleanup {
    param(
        [Parameter(Mandatory)][string]$RepoRoot,
        [switch]$Quiet
    )
    $LogRoot = Join-Path $env:TEMP "algorhythm-dev"
    $StackPidFile = Join-Path $LogRoot "stack_pids.json"

    if (-not $Quiet) {
        Write-Host "`n=== Stop dev stack (TEMP\algorhythm-dev, control-desktop) ===" -ForegroundColor Cyan
    }
    Stop-StackFromPidFile -Path $StackPidFile

    $cpPort = Get-ControlPlaneHttpPort -RepoRoot $RepoRoot
    if (Test-TcpListen -Port $cpPort) {
        Stop-ListenersOnPort -Port $cpPort
        try {
            Wait-PortFree -Port $cpPort -TimeoutSec 20
        } catch {
            Write-Warning $_.Exception.Message
        }
    }

    if ($Quiet) {
        Stop-ProcessesExecutableUnderCpAndMdiServices -RepoRoot $RepoRoot -Quiet
    } else {
        Stop-ProcessesExecutableUnderCpAndMdiServices -RepoRoot $RepoRoot
    }

    Get-Process -Name "control-desktop" -ErrorAction SilentlyContinue | ForEach-Object {
        if (-not $Quiet) {
            Write-Host "  Stop control-desktop (PID $($_.Id))" -ForegroundColor Yellow
        }
        Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue
    }

    if ($Quiet) {
        Stop-GoProcessesForRepo -RepoRoot $RepoRoot -Quiet
        Stop-ProcessesUsingAlgorhythmDevTemp -Quiet
        Stop-GoBuildTempExecutables -Quiet
    } else {
        Stop-GoProcessesForRepo -RepoRoot $RepoRoot
        Stop-ProcessesUsingAlgorhythmDevTemp
        Stop-GoBuildTempExecutables
    }

    Start-Sleep -Milliseconds 1500
}

function Test-FileLocked([string]$FilePath) {
    if (-not (Test-Path -LiteralPath $FilePath)) { return $false }
    try {
        $fs = [IO.File]::Open($FilePath, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
        $fs.Dispose()
        return $false
    } catch {
        return $true
    }
}

function Show-AlgorhythmDevLockSuspects {
    param(
        [string]$Hint,
        [string]$Path
    )
    Write-Host "  Hint: $Hint" -ForegroundColor DarkYellow

    if ($Path -and (Test-Path -LiteralPath $Path)) {
        Write-Host "  Files under ${Path}:" -ForegroundColor DarkYellow
        Get-ChildItem -LiteralPath $Path -File -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object {
            $locked = Test-FileLocked -FilePath $_.FullName
            $mark = if ($locked) { "LOCKED " } else { "free   " }
            Write-Host ("    {0} {1,10}  {2}" -f $mark, $_.Length, $_.FullName) -ForegroundColor DarkYellow
        }
    }

    $prefixes = Get-GoBuildCachePrefixes
    $namesOfInterest = @("go.exe", "api.exe", "worker.exe", "control-desktop.exe")
    $any = $false
    Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | ForEach-Object {
        $ex = $_.ExecutablePath
        $cl = $_.CommandLine
        $name = $_.Name
        $full = $null
        if (-not [string]::IsNullOrEmpty($ex)) {
            try { $full = [System.IO.Path]::GetFullPath($ex).ToLowerInvariant() } catch { $full = $null }
        }
        $match = $false
        if ($full) {
            foreach ($pref in $prefixes) {
                if ($full.StartsWith($pref)) { $match = $true; break }
            }
        }
        if (-not $match -and $cl -and $cl.ToLowerInvariant().Contains("algorhythm-dev")) { $match = $true }
        if (-not $match -and $namesOfInterest -contains $name) { $match = $true }
        if ($match) {
            $any = $true
            $shownEx = if ($ex) { $ex } else { $name }
            Write-Host ("  Still running: PID {0,-6} {1}" -f $_.ProcessId, $shownEx) -ForegroundColor DarkYellow
        }
    }
    if (-not $any) {
        Write-Host "  No go/api/worker/control-desktop processes in the system." -ForegroundColor DarkYellow
    }
}

function Remove-AlgorhythmDevTempTree {
    param(
        [Parameter(Mandatory)][string]$Path
    )
    if (-not (Test-Path -LiteralPath $Path)) { return $true }
    try {
        Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
        return $true
    } catch {
        try {
            $pending = Join-Path $env:TEMP ("algorhythm-dev.pending-delete-" + [guid]::NewGuid().ToString("n"))
            Move-Item -LiteralPath $Path -Destination $pending -Force -ErrorAction Stop
            Start-Sleep -Milliseconds 500
            try {
                Remove-Item -LiteralPath $pending -Recurse -Force -ErrorAction Stop
            } catch {
                # Standard path cleared; pending-delete-* may remain until reboot.
            }
            if (-not (Test-Path -LiteralPath $Path)) {
                return $true
            }
        } catch {
            # Move failed (handles still open); try robocopy wipe.
        }

        if (-not (Test-Path -LiteralPath $Path)) {
            return $true
        }

        $empty = Join-Path $env:TEMP ("algorhythm-empty-" + [guid]::NewGuid().ToString("n"))
        try {
            New-Item -ItemType Directory -Path $empty -Force | Out-Null
            $ea = [System.IO.Path]::GetFullPath($empty)
            $pa = [System.IO.Path]::GetFullPath($Path)
            $robCmd = ('robocopy "{0}" "{1}" /MIR /R:0 /W:0 /NFL /NDL /NJH /NJS /NP >nul 2>&1' -f $ea, $pa)
            cmd.exe /c $robCmd | Out-Null
            Remove-Item -LiteralPath $empty -Recurse -Force -ErrorAction SilentlyContinue
            Start-Sleep -Milliseconds 400
            Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
            return $true
        } catch {
            return $false
        }
    }
}
