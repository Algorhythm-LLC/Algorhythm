<#
.SYNOPSIS
  Короткая точка входа из корня репозитория: то же, что .\scripts\start-desktop-stack.ps1

.EXAMPLE
  .\scripts\run.ps1
  .\scripts\run.ps1 -SkipDocker -NoGui
#>
param(
    [switch]$SkipDocker,
    [switch]$SkipBuild,
    [switch]$NoGui,
    [switch]$NoStop,
    [int]$ReadyTimeoutSec = 180
)

& (Join-Path $PSScriptRoot "start-desktop-stack.ps1") @PSBoundParameters
