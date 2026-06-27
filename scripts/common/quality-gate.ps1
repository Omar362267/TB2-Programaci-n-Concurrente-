[CmdletBinding()]
param(
    [string]$EvidenceDirectory = "",
    [switch]$IncludeRace
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. (Join-Path $PSScriptRoot "evidence-common.ps1")

$repoRoot = Get-RepoRoot

if ([string]::IsNullOrWhiteSpace($EvidenceDirectory)) {
    $EvidenceDirectory = New-EvidenceRun -Name "quality_gate"
}

Push-Location $repoRoot

try {
    Save-EvidenceEnvironment -EvidenceDirectory $EvidenceDirectory

    $goFiles = @(
        Get-ChildItem `
            -Path (Join-Path $repoRoot "cmd"), (Join-Path $repoRoot "internal") `
            -Recurse `
            -Filter "*.go" `
            -File |
        Select-Object -ExpandProperty FullName
    )

    if ($goFiles.Count -eq 0) {
        throw "No se encontraron archivos .go en cmd ni internal."
    }

    Save-EvidenceText `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "go_files_checked.txt" `
        -Content ($goFiles -join [Environment]::NewLine) | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "gofmt_check" `
        -Command {
            $unformatted = @(& gofmt -l @goFiles)

            if ($unformatted.Count -gt 0) {
                $unformatted | ForEach-Object { $_ }
                throw "Hay archivos Go sin formato. Ejecute: gofmt -w <archivos>"
            }

            "Todos los archivos Go están correctamente formateados."
        } | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "go_vet" `
        -Command {
            go vet ./...
        } | Out-Null

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "go_test" `
        -Command {
            go test -count=1 ./...
        } | Out-Null

    if ($IncludeRace) {
        Invoke-EvidenceStep `
            -EvidenceDirectory $EvidenceDirectory `
            -Name "go_test_race" `
            -Command {
                go test -count=1 -race ./...
            } | Out-Null
    }
    else {
        Save-EvidenceText `
            -EvidenceDirectory $EvidenceDirectory `
            -FileName "go_test_race.txt" `
            -Content "Prueba de race detector no solicitada en esta ejecución." | Out-Null
    }

    Invoke-EvidenceStep `
        -EvidenceDirectory $EvidenceDirectory `
        -Name "go_build_commands" `
        -Command {
            go build ./cmd/...
        } | Out-Null

    $summary = [PSCustomObject]@{
        go_files_checked = $goFiles.Count
        gofmt            = "passed"
        go_vet           = "passed"
        go_test          = "passed"
        go_test_race     = if ($IncludeRace) { "passed" } else { "skipped" }
        go_build         = "passed"
        completed_at     = (Get-Date).ToString("o")
    }

    Save-EvidenceJson `
        -EvidenceDirectory $EvidenceDirectory `
        -FileName "quality_summary.json" `
        -Value $summary | Out-Null

    Write-Host ""
    Write-Host "Puerta de calidad aprobada."
    Write-Host "Archivos Go revisados: $($goFiles.Count)"
    Write-Host "Evidencia: $EvidenceDirectory"
}
finally {
    Pop-Location
}
