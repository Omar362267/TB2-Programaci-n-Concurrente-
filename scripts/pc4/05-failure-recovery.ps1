[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [string]$ApiBaseUrl = "http://127.0.0.1:8080",
    [int]$TimeoutSeconds = 90,
    [int]$PollSeconds = 3
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

if ($TimeoutSeconds -le 0) {
    throw "-TimeoutSeconds debe ser mayor que cero."
}

if ($PollSeconds -le 0) {
    throw "-PollSeconds debe ser mayor que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc4_failure_recovery"
}

function Get-HealthSnapshot {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri
    )

    return Invoke-RestMethod `
        -Method GET `
        -Uri "$Uri/health" `
        -TimeoutSec 15 `
        -ErrorAction Stop
}

function Wait-ForCluster {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ExpectedStatus,

        [Parameter(Mandatory = $true)]
        [int]$ExpectedAvailableNodes
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $attempts = @()
    $lastError = $null
    $lastHealth = $null

    while ((Get-Date) -lt $deadline) {
        try {
            $health = Get-HealthSnapshot -Uri $ApiBaseUrl
            $lastHealth = $health

            $attempts += [PSCustomObject]@{
                checked_at = (Get-Date).ToString("o")
                api_status = $health.status
                cluster_status = $health.cluster.status
                nodes_available = [int]$health.cluster.nodes_available
                nodes_total = [int]$health.cluster.nodes_total
                error = $null
            }

            if (
                $health.cluster.status -eq $ExpectedStatus -and
                [int]$health.cluster.nodes_available -eq $ExpectedAvailableNodes -and
                [int]$health.cluster.nodes_total -eq 4
            ) {
                return [PSCustomObject]@{
                    health = $health
                    attempts = $attempts
                }
            }

            $lastError = "Estado actual: $($health.cluster.status), nodos: $($health.cluster.nodes_available)/$($health.cluster.nodes_total)"
        }
        catch {
            $lastError = $_.Exception.Message

            $attempts += [PSCustomObject]@{
                checked_at = (Get-Date).ToString("o")
                api_status = $null
                cluster_status = $null
                nodes_available = $null
                nodes_total = $null
                error = $lastError
            }
        }

        Start-Sleep -Seconds $PollSeconds
    }

    throw "No se alcanzó estado '$ExpectedStatus' con $ExpectedAvailableNodes nodos en $TimeoutSeconds segundos. Último detalle: $lastError"
}

function Invoke-ExpectedTrainFailure {
    $request = [ordered]@{
        iterations = 1
        learning_rate = 1.0
        reset_model = $false
    }

    $requestJson = $request | ConvertTo-Json -Depth 10

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "rejected_train_request.json" `
        -Value $request `
        -Depth 10 | Out-Null

    try {
        $response = Invoke-RestMethod `
            -Method POST `
            -Uri "$ApiBaseUrl/v1/train" `
            -ContentType "application/json" `
            -Body $requestJson `
            -TimeoutSec 30 `
            -ErrorAction Stop

        return [PSCustomObject]@{
            succeeded = $true
            status_code = 200
            response = $response
            error = $null
        }
    }
    catch {
        $statusCode = $null

        if ($null -ne $_.Exception.Response) {
            try {
                $statusCode = [int]$_.Exception.Response.StatusCode
            }
            catch {
                $statusCode = $null
            }
        }

        $responseBody = $_.ErrorDetails.Message

        if ([string]::IsNullOrWhiteSpace($responseBody)) {
            $responseBody = $_.Exception.Message
        }

        return [PSCustomObject]@{
            succeeded = $false
            status_code = $statusCode
            response = $responseBody
            error = $_.Exception.Message
        }
    }
}

Push-Location (Get-RepoRoot)

$nodeStopped = $false

try {
    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "failure_recovery_parameters.txt" `
        -Content @"
api_base_url=$ApiBaseUrl
target_node=ml-node-4
timeout_seconds=$TimeoutSeconds
poll_seconds=$PollSeconds
expected_degraded_nodes=3
expected_ready_nodes=4
"@ | Out-Null

    $beforeHealth = Get-HealthSnapshot -Uri $ApiBaseUrl

    if (
        $beforeHealth.status -ne "ready" -or
        $beforeHealth.cluster.status -ne "ready" -or
        [int]$beforeHealth.cluster.nodes_available -ne 4
    ) {
        throw "El clúster debe iniciar ready con 4 nodos. Estado actual: $($beforeHealth.cluster.status), nodos: $($beforeHealth.cluster.nodes_available)/$($beforeHealth.cluster.nodes_total)"
    }

    $modelBefore = Invoke-RestMethod `
        -Method GET `
        -Uri "$ApiBaseUrl/v1/model" `
        -TimeoutSec 15 `
        -ErrorAction Stop

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_before_failure.json" `
        -Value $beforeHealth `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "model_before_failure.json" `
        -Value $modelBefore `
        -Depth 20 | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "stop_ml_node_4" `
        -Command {
            docker compose stop ml-node-4
        } | Out-Null

    $nodeStopped = $true

    $degraded = Wait-ForCluster `
        -ExpectedStatus "degraded" `
        -ExpectedAvailableNodes 3

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_degraded.json" `
        -Value $degraded.health `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_degraded_attempts.json" `
        -Value $degraded.attempts `
        -Depth 20 | Out-Null

    $rejectedTrain = Invoke-ExpectedTrainFailure

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "rejected_train_response.json" `
        -Value $rejectedTrain `
        -Depth 20 | Out-Null

    if ($rejectedTrain.succeeded) {
        throw "El entrenamiento fue aceptado con el clúster degradado; debió ser rechazado."
    }

    if ([int]$rejectedTrain.status_code -ne 503) {
        throw "El entrenamiento degradado devolvió HTTP $($rejectedTrain.status_code); se esperaba HTTP 503."
    }

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "start_ml_node_4" `
        -Command {
            docker compose start ml-node-4
        } | Out-Null

    $nodeStopped = $false

    $recovered = Wait-ForCluster `
        -ExpectedStatus "ready" `
        -ExpectedAvailableNodes 4

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_recovered.json" `
        -Value $recovered.health `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_recovered_attempts.json" `
        -Value $recovered.attempts `
        -Depth 20 | Out-Null

    $modelAfter = Invoke-RestMethod `
        -Method GET `
        -Uri "$ApiBaseUrl/v1/model" `
        -TimeoutSec 15 `
        -ErrorAction Stop

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "model_after_recovery.json" `
        -Value $modelAfter `
        -Depth 20 | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "compose_ps_after_recovery" `
        -Command {
            docker compose ps
        } | Out-Null

    $validation = [PSCustomObject]@{
        pre_failure_status = $beforeHealth.cluster.status
        pre_failure_nodes_available = [int]$beforeHealth.cluster.nodes_available
        degraded_status = $degraded.health.cluster.status
        degraded_nodes_available = [int]$degraded.health.cluster.nodes_available
        training_rejected = -not [bool]$rejectedTrain.succeeded
        rejected_train_http_status = [int]$rejectedTrain.status_code
        recovered_status = $recovered.health.cluster.status
        recovered_nodes_available = [int]$recovered.health.cluster.nodes_available
        model_version_before = [int]$modelBefore.version
        model_version_after = [int]$modelAfter.version
        model_unchanged_after_rejected_train = ([int]$modelBefore.version -eq [int]$modelAfter.version)
        generated_at = (Get-Date).ToString("o")
    }

    if (-not $validation.model_unchanged_after_rejected_train) {
        throw "La versión del modelo cambió pese a que el entrenamiento degradado fue rechazado."
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "failure_recovery_validation.json" `
        -Value $validation `
        -Depth 20 | Out-Null

    Write-Host ""
    Write-Host "Prueba de degradación y recuperación completada."
    Write-Host "Degraded: $($validation.degraded_nodes_available)/4 nodos"
    Write-Host "Entrenamiento rechazado: HTTP $($validation.rejected_train_http_status)"
    Write-Host "Recovered: $($validation.recovered_nodes_available)/4 nodos"
    Write-Host "Modelo preservado: $($validation.model_unchanged_after_rejected_train)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    if ($nodeStopped) {
        try {
            docker compose start ml-node-4 | Out-Null
        }
        catch {
            Write-Warning "No se pudo reiniciar ml-node-4 automáticamente: $($_.Exception.Message)"
        }
    }

    Pop-Location
}
