[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [string]$ApiBaseUrl = "http://127.0.0.1:8080"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc4_predict_cache_persistence"
}

function Invoke-JsonRequest {
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
    $bodyJson = $null

    if ($null -ne $Body) {
        $bodyJson = $Body | ConvertTo-Json -Depth 20

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

    try {
        if ($null -eq $Body) {
            $response = Invoke-RestMethod `
                -Method $Method `
                -Uri $uri `
                -TimeoutSec 30 `
                -ErrorAction Stop
        }
        else {
            $response = Invoke-RestMethod `
                -Method $Method `
                -Uri $uri `
                -ContentType "application/json" `
                -Body $bodyJson `
                -TimeoutSec 30 `
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

        throw
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName $ResponseFileName `
        -Value $response `
        -Depth 30 | Out-Null

    return $response
}

function Invoke-DockerComposeText {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$Arguments
    )

    $output = (& docker compose @Arguments 2>&1 | Out-String -Width 240)
    $exitCode = $LASTEXITCODE

    if ($exitCode -ne 0) {
        throw "docker compose $($Arguments -join ' ') terminó con código $exitCode.`n$output"
    }

    return $output
}

function Get-HttpFailureEvidence {
    param(
        [Parameter(Mandatory = $true)]
        [System.Management.Automation.ErrorRecord]$ErrorRecord
    )

    $statusCode = $null
    $responseBody = $null

    if ($null -ne $ErrorRecord.Exception.Response) {
        try {
            $statusCode = [int]$ErrorRecord.Exception.Response.StatusCode

            $reader = New-Object System.IO.StreamReader(
                $ErrorRecord.Exception.Response.GetResponseStream()
            )

            $responseBody = $reader.ReadToEnd()
            $reader.Close()
        }
        catch {
            $responseBody = $ErrorRecord.Exception.Message
        }
    }
    else {
        $responseBody = $ErrorRecord.Exception.Message
    }

    return [PSCustomObject]@{
        status_code = $statusCode
        error       = $ErrorRecord.Exception.Message
        response    = $responseBody
    }
}

Push-Location (Get-RepoRoot)

try {
    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "predict_cache_parameters.txt" `
        -Content @"
api_base_url=$ApiBaseUrl
redis_service=redis
mongo_service=mongo
mongo_database=pc4_energy
"@ | Out-Null

    $health = Invoke-JsonRequest `
        -Method "GET" `
        -Path "/health" `
        -ResponseFileName "predict_precheck_health.json" `
        -RequestFileName "predict_precheck_health_request.txt"

    $storage = Invoke-JsonRequest `
        -Method "GET" `
        -Path "/v1/storage/status" `
        -ResponseFileName "predict_precheck_storage.json" `
        -RequestFileName "predict_precheck_storage_request.txt"

    if (
        $health.status -ne "ready" -or
        $health.cluster.status -ne "ready" -or
        [int]$health.cluster.nodes_available -ne 4
    ) {
        throw "El clúster no está ready para predicciones."
    }

    if (
        -not [bool]$storage.mongo_enabled -or
        $storage.mongo_status -ne "ready" -or
        -not [bool]$storage.redis_enabled -or
        $storage.redis_status -ne "ready"
    ) {
        throw "MongoDB o Redis no están listos para la prueba de persistencia."
    }

    # Se limpian solo las claves de predicción para garantizar:
    # primera llamada = cache miss; segunda llamada idéntica = cache hit.
    $redisKeysBefore = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "redis",
        "redis-cli", "--scan",
        "--pattern", "pc4:prediction:*"
    )

    $keysToDelete = @(
        $redisKeysBefore -split "(`r`n|`n|`r)" |
        Where-Object { $_ -like "pc4:prediction:*" }
    )

    foreach ($keyToDelete in $keysToDelete) {
        Invoke-DockerComposeText -Arguments @(
            "exec", "-T", "redis",
            "redis-cli", "DEL", $keyToDelete.Trim()
        ) | Out-Null
    }

    $clearedKeysText = if ($keysToDelete.Count -gt 0) {
        $keysToDelete -join [Environment]::NewLine
    }
    else {
        "No había claves pc4:prediction:* para eliminar antes de la prueba."
    }

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "redis_prediction_keys_cleared.txt" `
        -Content $clearedKeysText | Out-Null

    $mongoCountBefore = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "mongo",
        "mongosh", "--quiet",
        "--eval",
        "db.getSiblingDB('pc4_energy').prediction_logs.countDocuments()"
    )

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "mongo_prediction_logs_before.txt" `
        -Content $mongoCountBefore | Out-Null

    $predictionInput = [ordered]@{
        features = [ordered]@{
            hour                  = 20
            day_of_week           = 6
            month                 = 12
            is_weekend            = 1
            voltage               = 240.8
            global_reactive_power = 0.15
            global_intensity      = 7.2
            sub_metering_1        = 0
            sub_metering_2        = 1
            sub_metering_3        = 18
            other_consumption     = 12.5
        }
    }

    $firstPrediction = Invoke-JsonRequest `
        -Method "POST" `
        -Path "/v1/predict" `
        -Body $predictionInput `
        -ResponseFileName "predict_first.json" `
        -RequestFileName "predict_valid_request.json"

    $secondPrediction = Invoke-JsonRequest `
        -Method "POST" `
        -Path "/v1/predict" `
        -Body $predictionInput `
        -ResponseFileName "predict_second.json" `
        -RequestFileName "predict_same_request.json"

    if (-not ($firstPrediction.PSObject.Properties.Name -contains "cache_hit")) {
        throw "La primera predicción no incluyó la propiedad cache_hit."
    }

    if (-not ($secondPrediction.PSObject.Properties.Name -contains "cache_hit")) {
        throw "La segunda predicción no incluyó la propiedad cache_hit."
    }

    if ([bool]$firstPrediction.cache_hit) {
        throw "La primera predicción debería ser cache_hit=false."
    }

    if (-not [bool]$secondPrediction.cache_hit) {
        throw "La segunda predicción idéntica debería ser cache_hit=true."
    }

    if (
        [double]$firstPrediction.probability_high_demand -ne
        [double]$secondPrediction.probability_high_demand
    ) {
        throw "La predicción cacheada no coincide con la primera predicción."
    }

    $missingFeatureInput = [ordered]@{
        features = [ordered]@{
            hour                  = 20
            day_of_week           = 6
            month                 = 12
            is_weekend            = 1
            voltage               = 240.8
            global_reactive_power = 0.15
            global_intensity      = 7.2
            sub_metering_1        = 0
            sub_metering_2        = 1
            sub_metering_3        = 18
        }
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "predict_missing_feature_request.json" `
        -Value $missingFeatureInput `
        -Depth 20 | Out-Null

    $expectedFailureObserved = $false

    try {
        $missingJson = $missingFeatureInput | ConvertTo-Json -Depth 20

        Invoke-RestMethod `
            -Method POST `
            -Uri "$ApiBaseUrl/v1/predict" `
            -ContentType "application/json" `
            -Body $missingJson `
            -TimeoutSec 30 `
            -ErrorAction Stop | Out-Null

        throw "La predicción sin other_consumption no falló como se esperaba."
    }
    catch {
        $failure = Get-HttpFailureEvidence -ErrorRecord $_

        if ($failure.status_code -eq $null) {
            throw
        }

        Save-EvidenceJson `
            -EvidenceDirectory $EvidenceDirectory `
            -FileName "predict_missing_feature_error.json" `
            -Value $failure `
            -Depth 20 | Out-Null

        $expectedFailureObserved = $true
    }

    if (-not $expectedFailureObserved) {
        throw "No se registró el error esperado de feature faltante."
    }

    $redisKeys = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "redis",
        "redis-cli", "--scan",
        "--pattern", "pc4:prediction:*"
    )

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "redis_prediction_keys.txt" `
        -Content $redisKeys | Out-Null

    $cacheKey = @(
        $redisKeys -split "(`r`n|`n|`r)" |
        Where-Object { $_ -like "pc4:prediction:*" }
    ) | Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($cacheKey)) {
        throw "No se encontró una clave pc4:prediction:* en Redis."
    }

    $redisTTL = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "redis",
        "redis-cli", "TTL", $cacheKey.Trim()
    )

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "redis_prediction_ttl.txt" `
        -Content $redisTTL | Out-Null

    $ttlSeconds = [int]($redisTTL.Trim())

    if ($ttlSeconds -le 0) {
        throw "TTL inválido para Redis: $ttlSeconds"
    }

    $mongoCountAfter = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "mongo",
        "mongosh", "--quiet",
        "--eval",
        "db.getSiblingDB('pc4_energy').prediction_logs.countDocuments()"
    )

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "mongo_prediction_logs_after.txt" `
        -Content $mongoCountAfter | Out-Null

    $mongoTrainingCount = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "mongo",
        "mongosh", "--quiet",
        "--eval",
        "db.getSiblingDB('pc4_energy').training_runs.countDocuments()"
    )

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "mongo_training_runs_count.txt" `
        -Content $mongoTrainingCount | Out-Null

    $mongoRecentPredictions = Invoke-DockerComposeText -Arguments @(
        "exec", "-T", "mongo",
        "mongosh", "--quiet",
        "--eval",
        "db.getSiblingDB('pc4_energy').prediction_logs.find().sort({created_at:-1}).limit(3).toArray()"
    )

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "mongo_recent_prediction_logs.txt" `
        -Content $mongoRecentPredictions | Out-Null

    $validation = [PSCustomObject]@{
        first_cache_hit                 = [bool]$firstPrediction.cache_hit
        second_cache_hit                = [bool]$secondPrediction.cache_hit
        probability_first               = [double]$firstPrediction.probability_high_demand
        probability_second              = [double]$secondPrediction.probability_high_demand
        model_version                   = [int]$firstPrediction.model_version
        predicted_class                 = [int]$firstPrediction.predicted_high_demand
        redis_key_found                 = $cacheKey.Trim()
        redis_ttl_seconds               = $ttlSeconds
        expected_missing_feature_error  = $expectedFailureObserved
        mongo_prediction_logs_before    = [int]$mongoCountBefore.Trim()
        mongo_prediction_logs_after     = [int]$mongoCountAfter.Trim()
        mongo_training_runs             = [int]$mongoTrainingCount.Trim()
        generated_at                    = (Get-Date).ToString("o")
    }

    if ($validation.mongo_prediction_logs_after -lt ($validation.mongo_prediction_logs_before + 2)) {
        throw "MongoDB no registró las dos predicciones esperadas."
    }

    if ($validation.mongo_training_runs -lt 2) {
        throw "MongoDB debería contener al menos los dos entrenamientos de esta ejecución."
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "predict_cache_persistence_validation.json" `
        -Value $validation `
        -Depth 20 | Out-Null

    Write-Host ""
    Write-Host "Predicción, cache y persistencia completadas."
    Write-Host "Primer predict cache_hit: $($validation.first_cache_hit)"
    Write-Host "Segundo predict cache_hit: $($validation.second_cache_hit)"
    Write-Host "TTL Redis: $($validation.redis_ttl_seconds) segundos"
    Write-Host "Mongo prediction_logs: $($validation.mongo_prediction_logs_before) -> $($validation.mongo_prediction_logs_after)"
    Write-Host "Mongo training_runs: $($validation.mongo_training_runs)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}



