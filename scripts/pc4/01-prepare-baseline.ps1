[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$LoaderWorkers = 8,
    [int]$Nodes = 4,
    [switch]$Overwrite
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

$repoRoot = Get-RepoRoot
$rawDataset = Join-Path $repoRoot "data\raw\household_power_consumption.txt"
$phase1Model = Join-Path $repoRoot "results\final_run_phase1\modelo_entrenado.json"
$outputDirectory = Join-Path $repoRoot "data\distributed"
$manifestPath = Join-Path $outputDirectory "manifest.json"

if ($LoaderWorkers -le 0) {
    throw "-LoaderWorkers debe ser mayor que cero."
}

if ($Nodes -le 0) {
    throw "-Nodes debe ser mayor que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc4_prepare_baseline"
}

Push-Location $repoRoot

try {
    if (-not (Test-Path $rawDataset)) {
        throw "No existe el dataset requerido: $rawDataset"
    }

    if (-not (Test-Path $phase1Model)) {
        throw "No existe el modelo base de PC3: $phase1Model"
    }

    if ((Test-Path $outputDirectory) -and -not $Overwrite) {
        throw "La carpeta ya existe: $outputDirectory. Ejecute reset-environment.ps1 o use -Overwrite."
    }

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "baseline_parameters.txt" `
        -Content @"
raw_dataset=$rawDataset
phase1_model=$phase1Model
output_directory=$outputDirectory
loader_workers=$LoaderWorkers
nodes=$Nodes
overwrite=$Overwrite
"@ | Out-Null

    $arguments = @(
        "run",
        "./cmd/shard-data",
        "-input", "data/raw/household_power_consumption.txt",
        "-model", "results/final_run_phase1/modelo_entrenado.json",
        "-out", "data/distributed",
        "-nodes", "$Nodes",
        "-workers", "$LoaderWorkers"
    )

    if ($Overwrite) {
        $arguments += "-overwrite=true"
    }

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "shard_data_generation" `
        -Command {
            & go @arguments
        } | Out-Null

    if (-not (Test-Path $manifestPath)) {
        throw "No se generó manifest.json en: $manifestPath"
    }

    $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json

    $shardSamples = @($manifest.shards | ForEach-Object { [int64]$_.samples })
    $sumShards = ($shardSamples | Measure-Object -Sum).Sum

    $validation = [PSCustomObject]@{
        schema_version                 = $manifest.schema_version
        generated_at                   = $manifest.generated_at
        valid_records                  = [int64]$manifest.valid_records
        train_samples                  = [int64]$manifest.train_samples
        test_samples                   = [int64]$manifest.test_samples
        shard_count                    = [int]$manifest.shard_count
        shard_samples_sum              = [int64]$sumShards
        train_coverage_matches         = [bool]$manifest.validation.train_coverage_matches
        normalizer_training_only       = [bool]$manifest.validation.normalizer_training_set_only
        difference_max_min_shard       = [int64]$manifest.validation.difference_max_min_shard
        train_sum_matches_manifest     = ([int64]$sumShards -eq [int64]$manifest.train_samples)
        expected_nodes_match_manifest  = ([int]$manifest.shard_count -eq $Nodes)
        feature_count                  = @($manifest.feature_names).Count
        test_file                      = $manifest.test_file
        test_sha256                    = $manifest.test_sha256
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "manifest.json" `
        -Value $manifest `
        -Depth 30 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "manifest_validation.json" `
        -Value $validation `
        -Depth 10 | Out-Null

    $shardSummary = $manifest.shards |
        Select-Object node_id, file, samples, positive_samples, negative_samples, sha256 |
        Format-Table -AutoSize |
        Out-String -Width 240

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "shard_summary.txt" `
        -Content $shardSummary | Out-Null

    if (-not $validation.train_sum_matches_manifest) {
        throw "La suma de shards ($sumShards) no coincide con train_samples ($($manifest.train_samples))."
    }

    if (-not $validation.train_coverage_matches) {
        throw "El manifest reporta train_coverage_matches=false."
    }

    if (-not $validation.normalizer_training_only) {
        throw "El manifest reporta normalizer_training_only=false."
    }

    if (-not $validation.expected_nodes_match_manifest) {
        throw "El manifest generó $($manifest.shard_count) shards, pero se solicitaron $Nodes."
    }

    Write-Host ""
    Write-Host "Baseline PC4 preparado correctamente."
    Write-Host "Evidencia: $EvidenceDirectory"
    Write-Host "Shards: $($manifest.shard_count)"
    Write-Host "Train/test: $($manifest.train_samples)/$($manifest.test_samples)"
}
finally {
    Pop-Location
}


