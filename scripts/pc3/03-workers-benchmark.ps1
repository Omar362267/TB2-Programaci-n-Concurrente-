[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [int]$Repetitions = 3,
    [string]$Workers = "1,2,4,8",
    [int]$Iterations = 300,
    [double]$LearningRate = 1.0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "..\common\evidence-common.ps1")

$repoRoot = Get-RepoRoot

if ($Repetitions -le 0) {
    throw "-Repetitions debe ser mayor que cero."
}

if ($Iterations -le 0) {
    throw "-Iterations debe ser mayor que cero."
}

if ($LearningRate -le 0) {
    throw "-LearningRate debe ser mayor que cero."
}

$workerValues = @(
    $Workers -split "," |
    ForEach-Object { $_.Trim() } |
    Where-Object { $_ -match "^\d+$" } |
    ForEach-Object { [int]$_ }
)

if ($workerValues.Count -lt 2) {
    throw "-Workers debe contener al menos dos valores enteros, por ejemplo: 1,2,4,8."
}

if ($workerValues | Where-Object { $_ -le 0 }) {
    throw "Todos los valores de -Workers deben ser mayores que cero."
}

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "pc3_workers_benchmark"
}

$benchmarkRoot = Join-Path $repoRoot "results\benchmarks_repetitions"
$aggregateCsv = Join-Path $EvidenceDirectory "pc3_workers_all_runs.csv"
$aggregateJson = Join-Path $EvidenceDirectory "pc3_workers_all_runs.json"
$medianCsv = Join-Path $EvidenceDirectory "pc3_workers_median.csv"
$medianJson = Join-Path $EvidenceDirectory "pc3_workers_median.json"
$summaryPath = Join-Path $EvidenceDirectory "pc3_workers_summary.md"

function Get-Median {
    param(
        [Parameter(Mandatory = $true)]
        [double[]]$Values
    )

    if ($Values.Count -eq 0) {
        throw "No se puede calcular mediana de una colección vacía."
    }

    $sorted = @($Values | Sort-Object)
    $middle = [int][math]::Floor($sorted.Count / 2)

    if (($sorted.Count % 2) -eq 1) {
        return [double]$sorted[$middle]
    }

    return [double](($sorted[$middle - 1] + $sorted[$middle]) / 2.0)
}

function Get-JsonNumber {
    param(
        [Parameter(Mandatory = $true)]
        [object]$Value
    )

    return [double]::Parse(
        [string]$Value,
        [System.Globalization.CultureInfo]::InvariantCulture
    )
}

Push-Location $repoRoot

try {
    New-Item -ItemType Directory -Path $benchmarkRoot -Force | Out-Null

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_workers_benchmark_parameters.txt" `
        -Content @"
workers=$Workers
repetitions=$Repetitions
iterations=$Iterations
learning_rate=$LearningRate
dataset=data/raw/household_power_consumption.txt
benchmark_root=$benchmarkRoot
"@ | Out-Null

    $allRows = @()
    $qualityRows = @()

    for ($repeat = 1; $repeat -le $Repetitions; $repeat++) {
        $runName = "run_{0:D2}" -f $repeat
        $runOutputRelative = "results/benchmarks_repetitions/$runName"
        $runOutputAbsolute = Join-Path $repoRoot $runOutputRelative
        $runFiguresRelative = "results/figures_repetitions/$runName"

        if (Test-Path $runOutputAbsolute) {
            Remove-Item -Path $runOutputAbsolute -Recurse -Force
        }

        Invoke-EvidenceStep `
            -EvidenceDirectory $EvidenceDirectory `
            -Name ("pc3_benchmark_" + $runName) `
            -Command {
                go run ./cmd/pc3-benchmark `
                    -input "data/raw/household_power_consumption.txt" `
                    -workers $Workers `
                    -iterations $Iterations `
                    -lr $LearningRate `
                    -out $runOutputRelative `
                    -figures $runFiguresRelative
            } | Out-Null

        $comparisonPath = Join-Path $runOutputAbsolute "benchmark_comparativo.json"

        if (-not (Test-Path $comparisonPath)) {
            throw "No se generó el comparativo esperado: $comparisonPath"
        }

        $comparison = Get-Content -Path $comparisonPath -Raw | ConvertFrom-Json
        $runRows = @($comparison.rows)

        if ($runRows.Count -ne $workerValues.Count) {
            throw "El benchmark $runName generó $($runRows.Count) filas; se esperaban $($workerValues.Count)."
        }

        foreach ($row in $runRows) {
            $allRows += [PSCustomObject]@{
                repetition = $repeat
                workers = [int]$row.workers
                rows_processed = [int64]$row.rows_processed
                load_duration_ms = [int64]$row.load_duration_ms
                feature_duration_ms = [int64]$row.feature_duration_ms
                train_duration_ms = [int64]$row.train_duration_ms
                total_duration_ms = [int64]$row.total_duration_ms
                rows_per_second = Get-JsonNumber -Value $row.rows_per_second
                original_speedup = Get-JsonNumber -Value $row.speedup
                original_efficiency = Get-JsonNumber -Value $row.efficiency
            }

            $metricsPath = Join-Path `
                $runOutputAbsolute `
                ("benchmark_w{0}\metricas_modelo.json" -f [int]$row.workers)

            if (-not (Test-Path $metricsPath)) {
                throw "No se generó métrica para worker $($row.workers): $metricsPath"
            }

            $metrics = Get-Content -Path $metricsPath -Raw | ConvertFrom-Json

            $qualityRows += [PSCustomObject]@{
                repetition = $repeat
                workers = [int]$row.workers
                train_samples = [int64]$metrics.train_samples
                test_samples = [int64]$metrics.test_samples
                accuracy = Get-JsonNumber -Value $metrics.test_metrics.accuracy
                precision = Get-JsonNumber -Value $metrics.test_metrics.precision
                recall = Get-JsonNumber -Value $metrics.test_metrics.recall
                f1_score = Get-JsonNumber -Value $metrics.test_metrics.f1_score
                initial_loss = Get-JsonNumber -Value $metrics.train_report.initial_loss
                final_loss = Get-JsonNumber -Value $metrics.train_report.final_loss
            }
        }

        Copy-Item `
            -Path $comparisonPath `
            -Destination (Join-Path $EvidenceDirectory ("benchmark_{0}.json" -f $runName)) `
            -Force
    }

    $allRows |
        Sort-Object repetition, workers |
        Export-Csv -Path $aggregateCsv -NoTypeInformation -Encoding UTF8

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_workers_all_runs.json" `
        -Value $allRows `
        -Depth 20 | Out-Null

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_workers_quality_all_runs.json" `
        -Value $qualityRows `
        -Depth 20 | Out-Null

    $baselineWorkers = ($workerValues | Sort-Object | Select-Object -First 1)

    $medianRows = @()

    foreach ($worker in ($workerValues | Sort-Object)) {
        $workerRuns = @($allRows | Where-Object { $_.workers -eq $worker })

        if ($workerRuns.Count -ne $Repetitions) {
            throw "El worker $worker tiene $($workerRuns.Count) repeticiones; se esperaban $Repetitions."
        }

        $medianRows += [PSCustomObject]@{
            workers = $worker
            repetitions = $workerRuns.Count
            rows_processed = [int64](Get-Median -Values @($workerRuns.rows_processed))
            load_duration_ms_median = [int64](Get-Median -Values @($workerRuns.load_duration_ms))
            feature_duration_ms_median = [int64](Get-Median -Values @($workerRuns.feature_duration_ms))
            train_duration_ms_median = [int64](Get-Median -Values @($workerRuns.train_duration_ms))
            total_duration_ms_median = [int64](Get-Median -Values @($workerRuns.total_duration_ms))
            rows_per_second_median = [math]::Round(
                (Get-Median -Values @($workerRuns.rows_per_second)),
                4
            )
        }
    }

    $baselineMedian = @(
        $medianRows |
        Where-Object { $_.workers -eq $baselineWorkers }
    )

    if ($baselineMedian.Count -ne 1) {
        throw "No se encontró una línea base única para worker $baselineWorkers."
    }

    foreach ($row in $medianRows) {
        $row | Add-Member -NotePropertyName speedup_median `
            -NotePropertyValue ([math]::Round(
                ($baselineMedian[0].total_duration_ms_median / [double]$row.total_duration_ms_median),
                4
            ))

        $row | Add-Member -NotePropertyName efficiency_median `
            -NotePropertyValue ([math]::Round(
                (($baselineMedian[0].total_duration_ms_median / [double]$row.total_duration_ms_median) / $row.workers),
                4
            ))
    }

    $medianRows |
        Sort-Object workers |
        Export-Csv -Path $medianCsv -NoTypeInformation -Encoding UTF8

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_workers_median.json" `
        -Value ($medianRows | Sort-Object workers) `
        -Depth 20 | Out-Null

    $qualityReference = @(
        $qualityRows |
        Where-Object {
            $_.repetition -eq 1 -and
            $_.workers -eq $baselineWorkers
        }
    )

    if ($qualityReference.Count -ne 1) {
        throw "No se encontró la métrica de referencia para validar consistencia."
    }

    $qualityTolerance = 0.0000000001

    foreach ($quality in $qualityRows) {
        if (
            $quality.train_samples -ne $qualityReference[0].train_samples -or
            $quality.test_samples -ne $qualityReference[0].test_samples -or
            [math]::Abs($quality.accuracy - $qualityReference[0].accuracy) -gt $qualityTolerance -or
            [math]::Abs($quality.precision - $qualityReference[0].precision) -gt $qualityTolerance -or
            [math]::Abs($quality.recall - $qualityReference[0].recall) -gt $qualityTolerance -or
            [math]::Abs($quality.f1_score - $qualityReference[0].f1_score) -gt $qualityTolerance -or
            [math]::Abs($quality.final_loss - $qualityReference[0].final_loss) -gt $qualityTolerance
        ) {
            throw "La calidad del modelo cambió entre configuraciones; revisar $($quality.repetition)/$($quality.workers)."
        }
    }

    $summaryLines = @(
        "# Benchmark de concurrencia PC3 con medianas",
        "",
        "Repeticiones por configuración: $Repetitions",
        "Workers evaluados: $Workers",
        "Iteraciones por ejecución: $Iterations",
        "",
        "| Workers | Repeticiones | Tiempo total mediano (ms) | Registros/s mediano | Speedup | Eficiencia |",
        "|---:|---:|---:|---:|---:|---:|"
    )

    foreach ($row in ($medianRows | Sort-Object workers)) {
        $summaryLines += (
            "| {0} | {1} | {2} | {3:N2} | {4:N2} | {5:N2} |" -f `
            $row.workers, `
            $row.repetitions, `
            $row.total_duration_ms_median, `
            $row.rows_per_second_median, `
            $row.speedup_median, `
            $row.efficiency_median
        )
    }

    $summaryLines += @(
        "",
        "La línea base para speedup y eficiencia es $baselineWorkers worker.",
        "Las métricas de calidad y la loss final fueron consistentes en todas las configuraciones evaluadas.",
        "La eficiencia puede disminuir al aumentar workers por costos de coordinación, planificación, memoria, CPU y E/S."
    )

    Set-Content -Path $summaryPath -Value $summaryLines -Encoding UTF8

    $validation = [PSCustomObject]@{
        repetitions_requested = $Repetitions
        workers_requested = $workerValues
        total_observations = $allRows.Count
        expected_observations = ($Repetitions * $workerValues.Count)
        all_observations_present = ($allRows.Count -eq ($Repetitions * $workerValues.Count))
        quality_consistent = $true
        baseline_workers = $baselineWorkers
        median_csv = $medianCsv
        median_json = $medianJson
        summary_markdown = $summaryPath
        generated_at = (Get-Date).ToString("o")
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "pc3_workers_benchmark_validation.json" `
        -Value $validation `
        -Depth 20 | Out-Null

    Write-Host ""
    Write-Host "Benchmark PC3 completado."
    Write-Host "Repeticiones: $Repetitions"
    Write-Host "Configuraciones: $Workers"
    Write-Host "Base de speedup: $baselineWorkers worker(s)"
    Write-Host "CSV de medianas: $medianCsv"
    Write-Host "Resumen: $summaryPath"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}
