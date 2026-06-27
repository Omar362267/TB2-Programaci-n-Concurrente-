[CmdletBinding()]
param(
    [switch]$KeepEvidence,
    [switch]$KeepDistributedData,
    [switch]$KeepDistributedResults
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "evidence-common.ps1")

$repoRoot = Get-RepoRoot
$rawDataset = Join-Path $repoRoot "data\raw\household_power_consumption.txt"
$phase1Model = Join-Path $repoRoot "results\final_run_phase1\modelo_entrenado.json"

function Remove-PathIfExists {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    if (Test-Path $Path) {
        Remove-Item -Path $Path -Recurse -Force
        Write-Host "[removed] $Path"
    }
    else {
        Write-Host "[skip] no existe: $Path"
    }
}

Push-Location $repoRoot

try {
    if (-not (Test-Path $rawDataset)) {
        throw "No existe el dataset requerido: $rawDataset"
    }

    if (-not (Test-Path $phase1Model)) {
        throw "No existe el artefacto base de PC3: $phase1Model. Debe regenerarse antes de preparar PC4."
    }

    Write-Host "Deteniendo Docker Compose y eliminando volúmenes del proyecto..."
    docker compose down -v --remove-orphans

    if (-not $KeepDistributedData) {
        Get-ChildItem (Join-Path $repoRoot "data") -Directory -ErrorAction SilentlyContinue |
            Where-Object {
                $_.Name -eq "distributed" -or
                $_.Name -like "distributed-*" -or
                $_.Name -like "distributed_*"
            } |
            ForEach-Object {
                Remove-PathIfExists -Path $_.FullName
            }
    }
    else {
        Write-Host "[keep] se conservan shards distribuidos"
    }

    if (-not $KeepDistributedResults) {
        Remove-PathIfExists -Path (Join-Path $repoRoot "results\distributed")
    }
    else {
        Write-Host "[keep] se conservan resultados distribuidos"
    }

    if (-not $KeepEvidence) {
        Remove-PathIfExists -Path (Join-Path $repoRoot "evidence")
        New-Item -ItemType Directory -Path (Join-Path $repoRoot "evidence") -Force | Out-Null
        Write-Host "[created] evidence"
    }
    else {
        Write-Host "[keep] se conservan evidencias"
    }

    Write-Host ""
    Write-Host "Entorno reiniciado correctamente."
    Write-Host "Conservado:"
    Write-Host "  - $rawDataset"
    Write-Host "  - $phase1Model"
    Write-Host ""
    Write-Host "Siguiente paso: ejecutar scripts\pc4\01-prepare-baseline.ps1"
}
finally {
    Pop-Location
}
