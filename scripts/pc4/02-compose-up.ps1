[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$TimeoutSeconds = 120,
    [int]$PollSeconds = 3,
    [switch]$SkipBuild
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

$repoRoot = Get-RepoRoot
$manifestPath = Join-Path $repoRoot "data\distributed\manifest.json"
$apiBaseUrl = "http://127.0.0.1:8080"

if ($TimeoutSeconds -le 0) {
    throw "-TimeoutSeconds debe ser mayor que cero."
}

if ($PollSeconds -le 0) {
    throw "-PollSeconds debe ser mayor que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc4_compose_up"
}

Push-Location $repoRoot

try {
    if (-not (Test-Path $manifestPath)) {
        throw "No existe $manifestPath. Ejecute primero scripts\pc4\01-prepare-baseline.ps1."
    }

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "compose_parameters.txt" `
        -Content @"
manifest=$manifestPath
api_base_url=$apiBaseUrl
timeout_seconds=$TimeoutSeconds
poll_seconds=$PollSeconds
skip_build=$SkipBuild
"@ | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "docker_info" `
        -Command {
            docker info
        } | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "docker_compose_config" `
        -Command {
            docker compose config
        } | Out-Null

    if ($SkipBuild) {
        Invoke-EvidenceStep `
            -EvidenceDirectory $EvidenceDirectory `
            -Name "docker_compose_up" `
            -Command {
                docker compose up -d
            } | Out-Null
    }
    else {
        Invoke-EvidenceStep `
            -EvidenceDirectory $EvidenceDirectory `
            -Name "docker_compose_up_build" `
            -Command {
                docker compose up --build -d
            } | Out-Null
    }

    $attempts = @()
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $health = $null
    $storage = $null
    $ready = $false
    $lastError = $null

    while ((Get-Date) -lt $deadline) {
        try {
            $health = Invoke-RestMethod `
                -Method GET `
                -Uri "$apiBaseUrl/health" `
                -TimeoutSec 8

            $attempts += [PSCustomObject]@{
                checked_at = (Get-Date).ToString("o")
                status     = $health.status
                cluster    = $health.cluster.status
                available  = $health.cluster.nodes_available
                total      = $health.cluster.nodes_total
                error      = $null
            }

            if (
                $health.status -eq "ready" -and
                $health.cluster.status -eq "ready" -and
                [int]$health.cluster.nodes_total -eq 4 -and
                [int]$health.cluster.nodes_available -eq 4
            ) {
                $storage = Invoke-RestMethod `
                    -Method GET `
                    -Uri "$apiBaseUrl/v1/storage/status" `
                    -TimeoutSec 8

                if (
                    $storage.mongo_status -eq "ready" -and
                    $storage.redis_status -eq "ready"
                ) {
                    $ready = $true
                    break
                }

                $lastError = "API lista, pero Mongo o Redis todavía no están ready."
            }
            else {
                $lastError = "La API respondió, pero el clúster aún no está listo."
            }
        }
        catch {
            $lastError = $_.Exception.Message

            $attempts += [PSCustomObject]@{
                checked_at = (Get-Date).ToString("o")
                status     = $null
                cluster    = $null
                available  = $null
                total      = $null
                error      = $lastError
            }
        }

        Start-Sleep -Seconds $PollSeconds
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health_attempts.json" `
        -Value $attempts `
        -Depth 20 | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "docker_compose_ps" `
        -Command {
            docker compose ps
        } | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "docker_compose_logs" `
        -Command {
            docker compose logs --no-color --tail 300
        } | Out-Null

    if (-not $ready) {
        throw "Docker Compose no alcanzó estado ready en $TimeoutSeconds segundos. Último detalle: $lastError"
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "health.json" `
        -Value $health `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "storage_status.json" `
        -Value $storage `
        -Depth 20 | Out-Null

    $validation = [PSCustomObject]@{
        api_status              = $health.status
        cluster_status          = $health.cluster.status
        nodes_total             = [int]$health.cluster.nodes_total
        nodes_available         = [int]$health.cluster.nodes_available
        total_samples           = [int64]$health.cluster.total_samples
        feature_count           = [int]$health.cluster.feature_count
        mongo_enabled           = [bool]$storage.mongo_enabled
        mongo_status            = $storage.mongo_status
        redis_enabled           = [bool]$storage.redis_enabled
        redis_status            = $storage.redis_status
        cache_ttl_seconds       = [int64]$storage.cache_ttl_seconds
        compose_ready           = $true
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "compose_validation.json" `
        -Value $validation `
        -Depth 20 | Out-Null

    Write-Host ""
    Write-Host "Docker Compose listo."
    Write-Host "API: $apiBaseUrl"
    Write-Host "Nodos disponibles: $($validation.nodes_available)/$($validation.nodes_total)"
    Write-Host "Muestras totales: $($validation.total_samples)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}
