[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$LoaderWorkers = 8,
    [int]$Nodes = 4,
    [int]$ShortIterations = 10,
    [int]$FinalIterations = 300,
    [double]$LearningRate = 1.0,
    [int]$ComposeTimeoutSeconds = 120,
    [int]$RecoveryTimeoutSeconds = 90,
    [int]$PollSeconds = 3,
    [switch]$SkipBuild,
    [switch]$FreshStart
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

if ($LoaderWorkers -le 0) {
    throw "-LoaderWorkers debe ser mayor que cero."
}

if ($Nodes -le 1) {
    throw "-Nodes debe ser mayor que uno."
}

if ($ShortIterations -le 0) {
    throw "-ShortIterations debe ser mayor que cero."
}

if ($FinalIterations -le 0) {
    throw "-FinalIterations debe ser mayor que cero."
}

if ($LearningRate -le 0) {
    throw "-LearningRate debe ser mayor que cero."
}

if ($ComposeTimeoutSeconds -le 0 -or $RecoveryTimeoutSeconds -le 0) {
    throw "Los tiempos de espera deben ser mayores que cero."
}

if ($PollSeconds -le 0) {
    throw "-PollSeconds debe ser mayor que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc4_complete"
}

$repoRoot = Get-RepoRoot
$apiBaseUrl = "http://127.0.0.1:8080"

$qualityEvidence = Join-Path $EvidenceDirectory "01_quality_gate"
$baselineEvidence = Join-Path $EvidenceDirectory "02_prepare_baseline"
$composeEvidence = Join-Path $EvidenceDirectory "03_compose_up"
$trainingEvidence = Join-Path $EvidenceDirectory "04_train_evaluate"
$predictionEvidence = Join-Path $EvidenceDirectory "05_predict_cache_persistence"
$recoveryEvidence = Join-Path $EvidenceDirectory "06_failure_recovery"

$qualityScript = Join-Path $PSScriptRoot "..\common\quality-gate.ps1"
$baselineScript = Join-Path $PSScriptRoot "01-prepare-baseline.ps1"
$composeScript = Join-Path $PSScriptRoot "02-compose-up.ps1"
$trainingScript = Join-Path $PSScriptRoot "03-train-evaluate.ps1"
$predictionScript = Join-Path $PSScriptRoot "04-predict-cache-persistence.ps1"
$recoveryScript = Join-Path $PSScriptRoot "05-failure-recovery.ps1"

$requiredScripts = @(
    $qualityScript,
    $baselineScript,
    $composeScript,
    $trainingScript,
    $predictionScript,
    $recoveryScript
)

foreach ($scriptPath in $requiredScripts) {
    if (-not (Test-Path $scriptPath)) {
        throw "No existe el script requerido: $scriptPath"
    }
}

function Get-DirectoryFileCount {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    if (-not (Test-Path $Path)) {
        return 0
    }

    return @(
        Get-ChildItem -Path $Path -Recurse -File -ErrorAction Stop
    ).Count
}

function Get-RequiredJson {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,

        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    try {
        return Invoke-RestMethod `
            -Method GET `
            -Uri $Uri `
            -TimeoutSec 20 `
            -ErrorAction Stop
    }
    catch {
        throw "No se pudo consultar $Name en $Uri. Detalle: $($_.Exception.Message)"
    }
}

foreach ($directory in @(
    $EvidenceDirectory,
    $qualityEvidence,
    $baselineEvidence,
    $composeEvidence,
    $trainingEvidence,
    $predictionEvidence,
    $recoveryEvidence
)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}

Push-Location $repoRoot

try {
    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc4_run_all_parameters.txt" `
        -Content @"
loader_workers=$LoaderWorkers
nodes=$Nodes
short_iterations=$ShortIterations
final_iterations=$FinalIterations
learning_rate=$LearningRate
compose_timeout_seconds=$ComposeTimeoutSeconds
recovery_timeout_seconds=$RecoveryTimeoutSeconds
poll_seconds=$PollSeconds
skip_build=$($SkipBuild.IsPresent)
fresh_start=$($FreshStart.IsPresent)
api_base_url=$apiBaseUrl
"@ | Out-Null

    if ($FreshStart.IsPresent) {
        Write-Host ""
        Write-Host "[0/6] Reiniciando servicios y volúmenes Docker..."

        Invoke-EvidenceStep `
            -EvidenceDirectory $EvidenceDirectory `
            -Name "docker_compose_down_fresh_start" `
            -Command {
                docker compose down -v --remove-orphans
            } | Out-Null
    }

    Write-Host ""
    Write-Host "[1/6] Ejecutando quality gate con pruebas de carrera..."

    & $qualityScript `
        -EvidenceDirectory $qualityEvidence `
        -IncludeRace

    if ((Get-DirectoryFileCount -Path $qualityEvidence) -eq 0) {
        throw "El quality gate no generó evidencia."
    }

    Write-Host ""
    Write-Host "[2/6] Preparando shards balanceados y manifest..."

    & $baselineScript `
        -EvidenceDirectory $baselineEvidence `
        -LoaderWorkers $LoaderWorkers `
        -Nodes $Nodes `
        -Overwrite

    $manifestPath = Join-Path $repoRoot "data\distributed\manifest.json"

    if (-not (Test-Path $manifestPath)) {
        throw "No se generó el manifest requerido: $manifestPath"
    }

    Write-Host ""
    Write-Host "[3/6] Levantando Docker Compose y verificando 4 nodos..."

    & $composeScript `
        -EvidenceDirectory $composeEvidence `
        -TimeoutSeconds $ComposeTimeoutSeconds `
        -PollSeconds $PollSeconds `
        -SkipBuild:$SkipBuild.IsPresent

    $healthAfterCompose = Get-RequiredJson `
        -Uri "$apiBaseUrl/health" `
        -Name "health del clúster"

    if (
        $healthAfterCompose.status -ne "ready" -or
        $healthAfterCompose.cluster.status -ne "ready" -or
        [int]$healthAfterCompose.cluster.nodes_total -ne $Nodes -or
        [int]$healthAfterCompose.cluster.nodes_available -ne $Nodes
    ) {
        throw "El clúster no quedó ready con $Nodes/$Nodes nodos después de Compose."
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_after_compose.json" `
        -Value $healthAfterCompose `
        -Depth 20 | Out-Null

    Write-Host ""
    Write-Host "[4/6] Entrenando y evaluando modelo distribuido..."

    & $trainingScript `
        -EvidenceDirectory $trainingEvidence `
        -ApiBaseUrl $apiBaseUrl `
        -ShortIterations $ShortIterations `
        -FinalIterations $FinalIterations `
        -LearningRate $LearningRate

    $modelAfterTraining = Get-RequiredJson `
        -Uri "$apiBaseUrl/v1/model" `
        -Name "modelo distribuido"

    $metricsAfterTraining = Get-RequiredJson `
        -Uri "$apiBaseUrl/v1/metrics" `
        -Name "métricas distribuidas"

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "model_after_training.json" `
        -Value $modelAfterTraining `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "metrics_after_training.json" `
        -Value $metricsAfterTraining `
        -Depth 20 | Out-Null

    if ([int]$modelAfterTraining.version -le 0) {
        throw "El modelo distribuido no reporta una versión válida."
    }

    $trainingSummaryPath = Join-Path $trainingEvidence "train_evaluate_summary.json"

    if (-not (Test-Path $trainingSummaryPath)) {
        throw "No se generó el resumen de entrenamiento/evaluación: $trainingSummaryPath"
    }

    $trainingSummary = Get-Content -Path $trainingSummaryPath -Raw | ConvertFrom-Json

    $finalTrainingProperty = $trainingSummary.PSObject.Properties["final_training"]
    $finalEvaluationProperty = $trainingSummary.PSObject.Properties["final_evaluation"]
    $finalModelProperty = $trainingSummary.PSObject.Properties["final_model"]

    if (
        $null -eq $finalTrainingProperty -or
        $null -eq $finalEvaluationProperty -or
        $null -eq $finalModelProperty
    ) {
        throw "train_evaluate_summary.json no contiene final_training, final_evaluation y final_model."
    }

    $finalTrainingSummary = $finalTrainingProperty.Value
    $finalEvaluationSummary = $finalEvaluationProperty.Value
    $finalModelSummary = $finalModelProperty.Value

    Write-Host ""
    Write-Host "[5/6] Validando predicción, cache Redis y persistencia MongoDB..."

    & $predictionScript `
        -EvidenceDirectory $predictionEvidence `
        -ApiBaseUrl $apiBaseUrl

    if ((Get-DirectoryFileCount -Path $predictionEvidence) -eq 0) {
        throw "La prueba de predicción/cache/persistencia no generó evidencia."
    }

    Write-Host ""
    Write-Host "[6/6] Validando degradación, rechazo de entrenamiento y recuperación..."

    & $recoveryScript `
        -EvidenceDirectory $recoveryEvidence `
        -ApiBaseUrl $apiBaseUrl `
        -TimeoutSeconds $RecoveryTimeoutSeconds `
        -PollSeconds $PollSeconds

    $finalHealth = Get-RequiredJson `
        -Uri "$apiBaseUrl/health" `
        -Name "health final del clúster"

    $finalClusterStatus = Get-RequiredJson `
        -Uri "$apiBaseUrl/v1/cluster/status" `
        -Name "estado final del clúster"

    $finalModel = Get-RequiredJson `
        -Uri "$apiBaseUrl/v1/model" `
        -Name "modelo final"

    $finalMetrics = Get-RequiredJson `
        -Uri "$apiBaseUrl/v1/metrics" `
        -Name "métricas finales"

    if (
        $finalHealth.status -ne "ready" -or
        $finalHealth.cluster.status -ne "ready" -or
        [int]$finalHealth.cluster.nodes_total -ne $Nodes -or
        [int]$finalHealth.cluster.nodes_available -ne $Nodes
    ) {
        throw "El clúster no quedó recuperado al finalizar la ejecución."
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_final.json" `
        -Value $finalHealth `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "cluster_status_final.json" `
        -Value $finalClusterStatus `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "model_final.json" `
        -Value $finalModel `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "metrics_final.json" `
        -Value $finalMetrics `
        -Depth 20 | Out-Null

    $validation = [PSCustomObject]@{
        quality_gate_completed = ((Get-DirectoryFileCount -Path $qualityEvidence) -gt 0)
        baseline_completed = ((Get-DirectoryFileCount -Path $baselineEvidence) -gt 0)
        compose_completed = ((Get-DirectoryFileCount -Path $composeEvidence) -gt 0)
        training_completed = ((Get-DirectoryFileCount -Path $trainingEvidence) -gt 0)
        prediction_cache_persistence_completed = ((Get-DirectoryFileCount -Path $predictionEvidence) -gt 0)
        failure_recovery_completed = ((Get-DirectoryFileCount -Path $recoveryEvidence) -gt 0)

        manifest_exists = (Test-Path $manifestPath)
        cluster_status_final = $finalHealth.cluster.status
        nodes_total_final = [int]$finalHealth.cluster.nodes_total
        nodes_available_final = [int]$finalHealth.cluster.nodes_available
        total_samples_final = [int64]$finalHealth.cluster.total_samples
        feature_count_final = [int]$finalHealth.cluster.feature_count

        model_version_final = [int]$finalModel.version
        model_feature_count_final = @($finalModel.feature_names).Count

        accuracy_final = [double]$finalEvaluationSummary.accuracy
        precision_final = [double]$finalEvaluationSummary.precision
        recall_final = [double]$finalEvaluationSummary.recall
        f1_score_final = [double]$finalEvaluationSummary.f1_score

                training_initial_loss = [double]$finalTrainingSummary.initial_loss
        training_final_loss = [double]$finalTrainingSummary.final_loss
        training_iterations = [int]$finalTrainingSummary.iterations
        training_nodes_used = [int]$finalTrainingSummary.nodes_used
        training_samples_per_iteration = [int64]$finalTrainingSummary.samples_per_iteration

        evaluation_loss = [double]$finalEvaluationSummary.loss
        evaluation_samples = [int64]$finalEvaluationSummary.samples
        evaluation_data_split = $finalEvaluationSummary.data_split
        true_positive = [int]$finalEvaluationSummary.true_positive
        true_negative = [int]$finalEvaluationSummary.true_negative
        false_positive = [int]$finalEvaluationSummary.false_positive
        false_negative = [int]$finalEvaluationSummary.false_negative

        generated_at = (Get-Date).ToString("o")
    }

    if ($validation.feature_count_final -ne 11) {
        throw "El clúster final no reporta las 11 features esperadas."
    }

    if ($validation.model_feature_count_final -ne 11) {
        throw "El modelo final no reporta 11 features."
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc4_run_all_validation.json" `
        -Value $validation `
        -Depth 20 | Out-Null

    Write-Host ""
    Write-Host "PC4 completa finalizada correctamente."
    Write-Host "Clúster final: $($validation.cluster_status_final) ($($validation.nodes_available_final)/$($validation.nodes_total_final) nodos)"
    Write-Host "Muestras: $($validation.total_samples_final)"
    Write-Host "Modelo: versión $($validation.model_version_final), $($validation.model_feature_count_final) features"
    Write-Host "F1 final: $($validation.f1_score_final)"
    Write-Host "Evidencia raíz: $EvidenceDirectory"
}
finally {
    Pop-Location
}

