[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$Workers = 8,
    [int]$Iterations = 300,
    [double]$LearningRate = 1.0,
    [double]$TestRatio = 0.2
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

$repoRoot = Get-RepoRoot
$datasetPath = Join-Path $repoRoot "data\raw\household_power_consumption.txt"
$runOutput = Join-Path $repoRoot "results\final_run_phase1"
$processedOutput = Join-Path $repoRoot "data\processed"

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

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc3_base_training"
}

function Copy-RunArtifact {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SourcePath,

        [Parameter(Mandatory = $true)]
        [string]$DestinationName
    )

    if (-not (Test-Path $SourcePath)) {
        throw "No se generó el artefacto esperado: $SourcePath"
    }

    $destination = Join-Path $EvidenceDirectory $DestinationName
    Copy-Item -Path $SourcePath -Destination $destination -Force
    return $destination
}

Push-Location $repoRoot

try {
    if (-not (Test-Path $datasetPath)) {
        throw "No existe el dataset requerido: $datasetPath"
    }

    New-Item -ItemType Directory -Path $runOutput -Force | Out-Null
    New-Item -ItemType Directory -Path $processedOutput -Force | Out-Null

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_base_parameters.txt" `
        -Content @"
dataset=$datasetPath
workers=$Workers
iterations=$Iterations
learning_rate=$LearningRate
test_ratio=$TestRatio
run_output=$runOutput
processed_output=$processedOutput
save_processed=false
"@ | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "pc3_base_training" `
        -Command {
            go run ./cmd/pc3 `
                -input "data/raw/household_power_consumption.txt" `
                -workers $Workers `
                -iterations $Iterations `
                -lr $LearningRate `
                -test-ratio $TestRatio `
                -out "results/final_run_phase1" `
                -processed-out "data/processed" `
                -save-processed=false
        } | Out-Null

    $parametersPath = Join-Path $runOutput "parametros_ejecucion.json"
    $cleaningPath = Join-Path $runOutput "resumen_limpieza.json"
    $metricsPath = Join-Path $runOutput "metricas_modelo.json"
    $modelPath = Join-Path $runOutput "modelo_entrenado.json"
    $benchmarkPath = Join-Path $runOutput "benchmark_concurrencia.json"
    $predictionsPath = Join-Path $runOutput "predicciones_muestra.csv"

    Copy-RunArtifact -SourcePath $parametersPath -DestinationName "pc3_execution_parameters.json" | Out-Null
    Copy-RunArtifact -SourcePath $cleaningPath -DestinationName "pc3_cleaning_summary.json" | Out-Null
    Copy-RunArtifact -SourcePath $metricsPath -DestinationName "pc3_model_metrics.json" | Out-Null
    Copy-RunArtifact -SourcePath $modelPath -DestinationName "pc3_model_artifact.json" | Out-Null
    Copy-RunArtifact -SourcePath $benchmarkPath -DestinationName "pc3_performance.json" | Out-Null
    Copy-RunArtifact -SourcePath $predictionsPath -DestinationName "pc3_prediction_sample.csv" | Out-Null

    $cleaning = Get-Content -Path $cleaningPath -Raw | ConvertFrom-Json
    $metrics = Get-Content -Path $metricsPath -Raw | ConvertFrom-Json
    $artifact = Get-Content -Path $modelPath -Raw | ConvertFrom-Json
    $performance = Get-Content -Path $benchmarkPath -Raw | ConvertFrom-Json

    $featureCount = @($artifact.feature_names).Count
    $weightCount = @($artifact.model.weights).Count
    $trainSamples = [int64]$metrics.train_samples
    $testSamples = [int64]$metrics.test_samples
    $metricSamples = [int64]$metrics.test_metrics.samples

    $validation = [PSCustomObject]@{
        total_rows = [int64]$cleaning.total_rows
        valid_rows = [int64]$cleaning.valid_rows
        invalid_rows = [int64]$cleaning.invalid_rows
        total_matches_valid_plus_invalid = (
            [int64]$cleaning.total_rows -eq
            ([int64]$cleaning.valid_rows + [int64]$cleaning.invalid_rows)
        )
        loader_workers = [int]$cleaning.workers
        concurrency_model = $cleaning.concurrency_model
        discard_policy = $cleaning.discard_policy
        discard_reasons = $cleaning.discard_reasons

        feature_count = $featureCount
        model_weight_count = $weightCount
        feature_count_matches_weights = ($featureCount -eq $weightCount)
        threshold_p75 = [double]$artifact.high_demand_threshold_p75
        decision_boundary = [double]$artifact.decision_boundary
        normalizer_fitted_on = $artifact.normalizer.fitted_on

        train_samples = $trainSamples
        test_samples = $testSamples
        split_matches_valid_rows = (
            ($trainSamples + $testSamples) -eq [int64]$cleaning.valid_rows
        )
        metric_samples_match_test = ($metricSamples -eq $testSamples)

        initial_loss = [double]$metrics.train_report.initial_loss
        final_loss = [double]$metrics.train_report.final_loss
        loss_decreased = (
            [double]$metrics.train_report.final_loss -lt
            [double]$metrics.train_report.initial_loss
        )

        accuracy = [double]$metrics.test_metrics.accuracy
        precision = [double]$metrics.test_metrics.precision
        recall = [double]$metrics.test_metrics.recall
        f1_score = [double]$metrics.test_metrics.f1_score
        true_positive = [int]$metrics.test_metrics.true_positive
        true_negative = [int]$metrics.test_metrics.true_negative
        false_positive = [int]$metrics.test_metrics.false_positive
        false_negative = [int]$metrics.test_metrics.false_negative

        load_duration_ms = [int64]$performance.performance.load_duration_ms
        feature_duration_ms = [int64]$performance.performance.feature_duration_ms
        train_duration_ms = [int64]$performance.performance.train_duration_ms
        total_duration_ms = [int64]$performance.performance.total_duration_ms
        rows_per_second = [double]$performance.performance.rows_per_second
        generated_at = (Get-Date).ToString("o")
    }

    if (-not $validation.total_matches_valid_plus_invalid) {
        throw "El total de filas no coincide con valid_rows + invalid_rows."
    }

    if ($validation.loader_workers -ne $Workers) {
        throw "El resumen de limpieza reporta $($validation.loader_workers) workers, se esperaban $Workers."
    }

    if (-not $validation.feature_count_matches_weights) {
        throw "La cantidad de features no coincide con los pesos del modelo."
    }

    if ($validation.feature_count -ne 11) {
        throw "Se esperaban 11 features y se obtuvieron $($validation.feature_count)."
    }

    if (-not $validation.split_matches_valid_rows) {
        throw "Train + test no coincide con los registros válidos."
    }

    if (-not $validation.metric_samples_match_test) {
        throw "Las muestras de métricas no coinciden con test_samples."
    }

    if (-not $validation.loss_decreased) {
        throw "La loss final no disminuyó respecto de la inicial."
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_base_validation.json" `
        -Value $validation `
        -Depth 30 | Out-Null

    Write-Host ""
    Write-Host "PC3 base completada."
    Write-Host "Validos/descartados: $($validation.valid_rows)/$($validation.invalid_rows)"
    Write-Host "Train/test: $($validation.train_samples)/$($validation.test_samples)"
    Write-Host "Loss: $($validation.initial_loss) -> $($validation.final_loss)"
    Write-Host "Accuracy: $($validation.accuracy)"
    Write-Host "Precision: $($validation.precision)"
    Write-Host "Recall: $($validation.recall)"
    Write-Host "F1: $($validation.f1_score)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}

