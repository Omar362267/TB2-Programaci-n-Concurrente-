[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$Workers = 8,
    [int]$Iterations = 300,
    [double]$LearningRate = 1.0,
    [double]$TestRatio = 0.2,
    [int]$BenchmarkRepetitions = 3,
    [string]$BenchmarkWorkers = "1,2,4,8",
    [switch]$SkipBenchmark
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

if ($Workers -le 0) {
    throw "-Workers debe ser mayor que cero."
}

if ($Iterations -le 0) {
    throw "-Iterations debe ser mayor que cero."
}

if ($LearningRate -le 0) {
    throw "-LearningRate debe ser mayor que cero."
}

if ($TestRatio -le 0 -or $TestRatio -ge 0.5) {
    throw "-TestRatio debe estar entre 0 y 0.5."
}

if ($BenchmarkRepetitions -le 0) {
    throw "-BenchmarkRepetitions debe ser mayor que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc3_complete"
}

$repoRoot = Get-RepoRoot

$baseEvidence = Join-Path $EvidenceDirectory "01_base_training"
$benchmarkEvidence = Join-Path $EvidenceDirectory "02_workers_benchmark"
$analysisEvidence = Join-Path $EvidenceDirectory "03_analysis"

$baseScript = Join-Path $PSScriptRoot "02-cleaning-and-base-training.ps1"
$benchmarkScript = Join-Path $PSScriptRoot "03-workers-benchmark.ps1"
$analysisScript = Join-Path $PSScriptRoot "04-analysis.ps1"

foreach ($scriptPath in @($baseScript, $benchmarkScript, $analysisScript)) {
    if (-not (Test-Path $scriptPath)) {
        throw "No existe el script requerido: $scriptPath"
    }
}

foreach ($directory in @($EvidenceDirectory, $baseEvidence, $benchmarkEvidence, $analysisEvidence)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}

Push-Location $repoRoot

try {
    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_run_all_parameters.txt" `
        -Content @"
workers=$Workers
iterations=$Iterations
learning_rate=$LearningRate
test_ratio=$TestRatio
benchmark_repetitions=$BenchmarkRepetitions
benchmark_workers=$BenchmarkWorkers
skip_benchmark=$($SkipBenchmark.IsPresent)
"@ | Out-Null

    Write-Host ""
    Write-Host "[1/3] Ejecutando limpieza, features y entrenamiento base..."

    & $baseScript `
        -EvidenceDirectory $baseEvidence `
        -Workers $Workers `
        -Iterations $Iterations `
        -LearningRate $LearningRate `
        -TestRatio $TestRatio


    $baseValidation = Join-Path $baseEvidence "pc3_base_validation.json"

    if (-not (Test-Path $baseValidation)) {
        throw "No se generó pc3_base_validation.json."
    }

    if ($SkipBenchmark.IsPresent) {
        Write-Host ""
        Write-Host "[2/3] Benchmark omitido por -SkipBenchmark."
    }
    else {
        Write-Host ""
        Write-Host "[2/3] Ejecutando benchmark con medianas..."

        & $benchmarkScript `
            -EvidenceDirectory $benchmarkEvidence `
            -Repetitions $BenchmarkRepetitions `
            -Workers $BenchmarkWorkers `
            -Iterations $Iterations `
            -LearningRate $LearningRate


        $benchmarkValidation = Join-Path $benchmarkEvidence "pc3_workers_benchmark_validation.json"

        if (-not (Test-Path $benchmarkValidation)) {
            throw "No se generó pc3_workers_benchmark_validation.json."
        }
    }

    Write-Host ""
    Write-Host "[3/3] Generando análisis y gráficos SVG..."

    & $analysisScript `
        -EvidenceDirectory $analysisEvidence `
        -Workers $Workers


    $analysisValidation = Join-Path $analysisEvidence "pc3_analysis_validation.json"

    if (-not (Test-Path $analysisValidation)) {
        throw "No se generó pc3_analysis_validation.json."
    }

    $finalValidation = [PSCustomObject]@{
        base_training_completed = $true
        workers_benchmark_completed = (-not $SkipBenchmark.IsPresent)
        analysis_completed = $true
        base_validation = $baseValidation
        benchmark_validation = if ($SkipBenchmark.IsPresent) { $null } else { $benchmarkValidation }
        analysis_validation = $analysisValidation
        generated_at = (Get-Date).ToString("o")
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_run_all_validation.json" `
        -Value $finalValidation `
        -Depth 10 | Out-Null

    Write-Host ""
    Write-Host "PC3 completa finalizada correctamente."
    Write-Host "Evidencia raíz: $EvidenceDirectory"
    Write-Host "Base: $baseEvidence"

    if (-not $SkipBenchmark.IsPresent) {
        Write-Host "Benchmark: $benchmarkEvidence"
    }

    Write-Host "Análisis: $analysisEvidence"
}
finally {
    Pop-Location
}

