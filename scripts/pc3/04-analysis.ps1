[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$Workers = 8
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

if ($Workers -le 0) {
    throw "-Workers debe ser mayor que cero."
}

$repoRoot = Get-RepoRoot
$datasetRelative = "data/raw/household_power_consumption.txt"
$metricsRelative = "results/final_run_phase1/metricas_modelo.json"
$analysisRelative = "results/analysis_final"
$figuresRelative = "results/figures_analysis_final"

$datasetPath = Join-Path $repoRoot $datasetRelative
$metricsPath = Join-Path $repoRoot $metricsRelative
$analysisPath = Join-Path $repoRoot $analysisRelative
$figuresPath = Join-Path $repoRoot $figuresRelative

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc3_analysis"
}

function Get-ArtifactManifest {
    param(
        [Parameter(Mandatory = $true)]
        [string]$RootPath
    )

    if (-not (Test-Path $RootPath)) {
        return @()
    }

    return @(
        Get-ChildItem -Path $RootPath -Recurse -File |
        Sort-Object FullName |
        ForEach-Object {
            [PSCustomObject]@{
                relative_path = $_.FullName.Replace($RootPath, "").TrimStart("\")
                bytes = [int64]$_.Length
            }
        }
    )
}

function Copy-DirectoryContents {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Source,

        [Parameter(Mandatory = $true)]
        [string]$Destination
    )

    if (Test-Path $Destination) {
        Remove-Item -Path $Destination -Recurse -Force
    }

    New-Item -ItemType Directory -Path $Destination -Force | Out-Null

    Get-ChildItem -Path $Source -Force |
        Copy-Item -Destination $Destination -Recurse -Force
}

Push-Location $repoRoot

try {
    if (-not (Test-Path $datasetPath)) {
        throw "No existe el dataset requerido: $datasetPath"
    }

    if (-not (Test-Path $metricsPath)) {
        throw "No existe la métrica base requerida: $metricsPath"
    }

    if (Test-Path $analysisPath) {
        Remove-Item -Path $analysisPath -Recurse -Force
    }

    if (Test-Path $figuresPath) {
        Remove-Item -Path $figuresPath -Recurse -Force
    }

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_analysis_parameters.txt" `
        -Content @"
dataset=$datasetRelative
metrics=$metricsRelative
workers=$Workers
analysis_output=$analysisRelative
figures_output=$figuresRelative
"@ | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "pc3_analysis" `
        -Command {
            go run ./cmd/pc3-analysis `
                -input $datasetRelative `
                -metrics $metricsRelative `
                -out $analysisRelative `
                -figures $figuresRelative `
                -workers $Workers
        } | Out-Null

    $requiredTableFiles = @(
        "analisis_resumen.md",
        "consumo_por_hora.csv",
        "distribucion_high_demand.json",
        "matriz_correlacion.csv"
    )

    $requiredFigureFiles = @(
        "curva_loss.svg",
        "matriz_confusion.svg"
    )

    foreach ($fileName in $requiredTableFiles) {
        $expectedPath = Join-Path $analysisPath $fileName

        if (-not (Test-Path $expectedPath)) {
            throw "No se generó la tabla o resumen requerido: $expectedPath"
        }
    }

    foreach ($fileName in $requiredFigureFiles) {
        $expectedPath = Join-Path $figuresPath $fileName

        if (-not (Test-Path $expectedPath)) {
            throw "No se generó el gráfico requerido: $expectedPath"
        }
    }

    $tablesManifest = Get-ArtifactManifest -RootPath $analysisPath
    $figuresManifest = Get-ArtifactManifest -RootPath $figuresPath

    if ($tablesManifest.Count -eq 0) {
        throw "No se generaron archivos de análisis."
    }

    if ($figuresManifest.Count -eq 0) {
        throw "No se generaron gráficos SVG."
    }

    $evidenceTablesPath = Join-Path $EvidenceDirectory "analysis_tables"
    $evidenceFiguresPath = Join-Path $EvidenceDirectory "analysis_figures"

    Copy-DirectoryContents `
        -Source $analysisPath `
        -Destination $evidenceTablesPath

    Copy-DirectoryContents `
        -Source $figuresPath `
        -Destination $evidenceFiguresPath

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_analysis_tables_manifest.json" `
        -Value $tablesManifest `
        -Depth 10 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_analysis_figures_manifest.json" `
        -Value $figuresManifest `
        -Depth 10 | Out-Null

    $validation = [PSCustomObject]@{
        workers = $Workers
        table_count = $tablesManifest.Count
        figure_count = $figuresManifest.Count
        required_tables_present = $true
        required_figures_present = $true
        analysis_evidence_path = $evidenceTablesPath
        figures_evidence_path = $evidenceFiguresPath
        generated_at = (Get-Date).ToString("o")
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_analysis_validation.json" `
        -Value $validation `
        -Depth 10 | Out-Null

    Write-Host ""
    Write-Host "Análisis PC3 completado."
    Write-Host "Tablas y resúmenes: $($validation.table_count)"
    Write-Host "Gráficos SVG: $($validation.figure_count)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}
