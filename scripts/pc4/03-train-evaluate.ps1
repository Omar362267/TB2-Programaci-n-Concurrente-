[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [string]$ApiBaseUrl = "http://127.0.0.1:8080",
    [int]$ShortIterations = 10,
    [int]$FinalIterations = 300,
    [double]$LearningRate = 1.0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

if ($ShortIterations -le 0) {
    throw "-ShortIterations debe ser mayor que cero."
}

if ($FinalIterations -le 0 -or $FinalIterations -gt 1000) {
    throw "-FinalIterations debe estar entre 1 y 1000."
}

if ($LearningRate -le 0) {
    throw "-LearningRate debe ser mayor que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc4_train_evaluate"
}

function Invoke-ApiJson {
    param(
        [Parameter(Mandatory = $true)]
        [ValidateSet("GET", "POST")]
        [string]$Method,

        [Parameter(Mandatory = $true)]
        [string]$Path,

        [object]$Body = $null,

        [Parameter(Mandatory = $true)]
        [string]$ResponseFileName,

        [Parameter(Mandatory = $true)]
        [string]$RequestFileName
    )

    $uri = "$ApiBaseUrl$Path"
    $requestBody = $null

    if ($null -ne $Body) {
        $requestBody = $Body | ConvertTo-Json -Depth 20
        Save-EvidenceJson `
            -EvidenceDirectory $EvidenceDirectory `
            -FileName $RequestFileName `
            -Value $Body `
            -Depth 20 | Out-Null
    }
    else {
        Save-EvidenceText `
            -EvidenceDirectory $EvidenceDirectory `
            -FileName $RequestFileName `
            -Content "Sin body; endpoint invocado mediante $Method." | Out-Null
    }

    $startedAt = Get-Date
    $response = $null

    try {
        if ($null -eq $Body) {
            $response = Invoke-RestMethod `
                -Method $Method `
                -Uri $uri `
                -TimeoutSec 180 `
                -ErrorAction Stop
        }
        else {
            $response = Invoke-RestMethod `
                -Method $Method `
                -Uri $uri `
                -ContentType "application/json" `
                -Body $requestBody `
                -TimeoutSec 180 `
                -ErrorAction Stop
        }
    }
    catch {
        $failure = [PSCustomObject]@{
            endpoint    = $uri
            method      = $Method
            failed_at   = (Get-Date).ToString("o")
            duration_ms = [math]::Round(((Get-Date) - $startedAt).TotalMilliseconds, 0)
            error       = $_.Exception.Message
        }

        Save-EvidenceJson `
            -EvidenceDirectory $EvidenceDirectory `
            -FileName ($ResponseFileName -replace "\.json$", "_error.json") `
            -Value $failure `
            -Depth 20 | Out-Null

        throw "Falló $Method $uri : $($_.Exception.Message)"
    }

    $metadata = [PSCustomObject]@{
        endpoint    = $uri
        method      = $Method
        requested_at = $startedAt.ToString("o")
        duration_ms = [math]::Round(((Get-Date) - $startedAt).TotalMilliseconds, 0)
        success     = $true
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName $ResponseFileName `
        -Value $response `
        -Depth 30 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName ($ResponseFileName -replace "\.json$", "_client_meta.json") `
        -Value $metadata `
        -Depth 20 | Out-Null

    return $response
}

Push-Location (Get-RepoRoot)

try {
    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "train_evaluate_parameters.txt" `
        -Content @"
api_base_url=$ApiBaseUrl
short_iterations=$ShortIterations
final_iterations=$FinalIterations
learning_rate=$LearningRate
short_training_reset_model=true
final_training_reset_model=true
"@ | Out-Null

    $health = Invoke-ApiJson `
        -Method "GET" `
        -Path "/health" `
        -ResponseFileName "pretrain_health.json" `
        -RequestFileName "pretrain_health_request.txt"

    if (
        $health.status -ne "ready" -or
        $health.cluster.status -ne "ready" -or
        [int]$health.cluster.nodes_available -ne 4 -or
        [int]$health.cluster.nodes_total -ne 4
    ) {
        throw "El clúster no está listo para entrenar. Estado: $($health.cluster.status), nodos: $($health.cluster.nodes_available)/$($health.cluster.nodes_total)"
    }

    $shortRequest = [ordered]@{
        iterations    = $ShortIterations
        learning_rate = $LearningRate
        reset_model   = $true
    }

    $shortTraining = Invoke-ApiJson `
        -Method "POST" `
        -Path "/v1/train" `
        -Body $shortRequest `
        -ResponseFileName "train_short.json" `
        -RequestFileName "train_short_request.json"

    $shortEvaluation = Invoke-ApiJson `
        -Method "POST" `
        -Path "/v1/evaluate" `
        -ResponseFileName "evaluate_short.json" `
        -RequestFileName "evaluate_short_request.txt"

    $finalRequest = [ordered]@{
        iterations    = $FinalIterations
        learning_rate = $LearningRate
        reset_model   = $true
    }

    $finalTraining = Invoke-ApiJson `
        -Method "POST" `
        -Path "/v1/train" `
        -Body $finalRequest `
        -ResponseFileName "train_final.json" `
        -RequestFileName "train_final_request.json"

    $finalEvaluation = Invoke-ApiJson `
        -Method "POST" `
        -Path "/v1/evaluate" `
        -ResponseFileName "evaluate_final.json" `
        -RequestFileName "evaluate_final_request.txt"

    $model = Invoke-ApiJson `
        -Method "GET" `
        -Path "/v1/model" `
        -ResponseFileName "model_final.json" `
        -RequestFileName "model_final_request.txt"

    $metrics = Invoke-ApiJson `
        -Method "GET" `
        -Path "/v1/metrics" `
        -ResponseFileName "metrics_final.json" `
        -RequestFileName "metrics_final_request.txt"

    $summary = [PSCustomObject]@{
        short_training = [PSCustomObject]@{
            model_version          = [int]$shortTraining.model.version
            iterations             = [int]$shortTraining.training.iterations
            duration_ms            = [int64]$shortTraining.training.duration_ms
            initial_loss           = [double]$shortTraining.training.initial_loss
            final_loss             = [double]$shortTraining.training.final_loss
            samples_per_iteration  = [int64]$shortTraining.training.samples_per_iteration
            nodes_used             = [int]$shortTraining.training.nodes_used
        }
        short_evaluation = [PSCustomObject]@{
            model_version = [int]$shortEvaluation.evaluation.model_version
            loss          = [double]$shortEvaluation.evaluation.loss
            accuracy      = [double]$shortEvaluation.evaluation.classification.accuracy
            precision     = [double]$shortEvaluation.evaluation.classification.precision
            recall        = [double]$shortEvaluation.evaluation.classification.recall
            f1_score      = [double]$shortEvaluation.evaluation.classification.f1_score
            true_positive = [int]$shortEvaluation.evaluation.classification.true_positive
            true_negative = [int]$shortEvaluation.evaluation.classification.true_negative
            false_positive = [int]$shortEvaluation.evaluation.classification.false_positive
            false_negative = [int]$shortEvaluation.evaluation.classification.false_negative
            samples       = [int]$shortEvaluation.evaluation.samples
        }
        final_training = [PSCustomObject]@{
            model_version          = [int]$finalTraining.model.version
            iterations             = [int]$finalTraining.training.iterations
            duration_ms            = [int64]$finalTraining.training.duration_ms
            initial_loss           = [double]$finalTraining.training.initial_loss
            final_loss             = [double]$finalTraining.training.final_loss
            samples_per_iteration  = [int64]$finalTraining.training.samples_per_iteration
            nodes_used             = [int]$finalTraining.training.nodes_used
            reset_model            = $true
        }
        final_evaluation = [PSCustomObject]@{
            model_version = [int]$finalEvaluation.evaluation.model_version
            loss          = [double]$finalEvaluation.evaluation.loss
            accuracy      = [double]$finalEvaluation.evaluation.classification.accuracy
            precision     = [double]$finalEvaluation.evaluation.classification.precision
            recall        = [double]$finalEvaluation.evaluation.classification.recall
            f1_score      = [double]$finalEvaluation.evaluation.classification.f1_score
            true_positive = [int]$finalEvaluation.evaluation.classification.true_positive
            true_negative = [int]$finalEvaluation.evaluation.classification.true_negative
            false_positive = [int]$finalEvaluation.evaluation.classification.false_positive
            false_negative = [int]$finalEvaluation.evaluation.classification.false_negative
            samples       = [int]$finalEvaluation.evaluation.samples
            data_split    = $finalEvaluation.evaluation.data_split
        }
        final_model = [PSCustomObject]@{
            version           = [int]$model.version
            ready             = [bool]$model.ready
            feature_count     = @($model.feature_names).Count
            decision_boundary = [double]$model.decision_boundary
            artifact_path     = $model.artifact_path
        }
        generated_at = (Get-Date).ToString("o")
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "train_evaluate_summary.json" `
        -Value $summary `
        -Depth 30 | Out-Null

    Write-Host ""
    Write-Host "Entrenamiento y evaluación completados."
    Write-Host "Short: iteraciones=$($summary.short_training.iterations), loss=$($summary.short_training.final_loss), F1=$($summary.short_evaluation.f1_score)"
    Write-Host "Final: iteraciones=$($summary.final_training.iterations), loss=$($summary.final_training.final_loss), F1=$($summary.final_evaluation.f1_score)"
    Write-Host "Modelo final: versión $($summary.final_model.version)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}
