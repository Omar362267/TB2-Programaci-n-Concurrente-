Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:RepoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)

function Get-RepoRoot {
    return $script:RepoRoot
}

function New-EvidenceRun {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
    $safeName = $Name -replace "[^a-zA-Z0-9_-]", "_"

    $runDirectory = Join-Path $script:RepoRoot "evidence\$timestamp`_$safeName"

    New-Item -ItemType Directory -Path $runDirectory -Force | Out-Null
    return $runDirectory
}

function Ensure-EvidenceDirectory {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )

    if (-not (Test-Path $Path)) {
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
    }
}

function Save-EvidenceText {
    param(
        [Parameter(Mandatory = $true)]
        [string]$EvidenceDirectory,

        [Parameter(Mandatory = $true)]
        [string]$FileName,

        [Parameter(Mandatory = $true)]
        [string]$Content
    )

    Ensure-EvidenceDirectory -Path $EvidenceDirectory

    $path = Join-Path $EvidenceDirectory $FileName
    $Content | Out-File -FilePath $path -Encoding utf8
    return $path
}

function Save-EvidenceJson {
    param(
        [Parameter(Mandatory = $true)]
        [string]$EvidenceDirectory,

        [Parameter(Mandatory = $true)]
        [string]$FileName,

        [Parameter(Mandatory = $true)]
        [object]$Value,

        [int]$Depth = 20
    )

    Ensure-EvidenceDirectory -Path $EvidenceDirectory

    $path = Join-Path $EvidenceDirectory $FileName
    $Value | ConvertTo-Json -Depth $Depth | Out-File -FilePath $path -Encoding utf8
    return $path
}

function Invoke-EvidenceStep {
    param(
        [Parameter(Mandatory = $true)]
        [string]$EvidenceDirectory,

        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $true)]
        [scriptblock]$Command,

        [switch]$AllowFailure
    )

    Ensure-EvidenceDirectory -Path $EvidenceDirectory

    $safeName = $Name -replace "[^a-zA-Z0-9_-]", "_"
    $outputPath = Join-Path $EvidenceDirectory "$safeName.txt"
    $metadataPath = Join-Path $EvidenceDirectory "$safeName.meta.json"

    $startedAt = Get-Date
    $status = "passed"
    $exitCode = 0
    $output = ""
    $errorMessage = $null

    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $global:LASTEXITCODE = 0

    try {
        $output = (& $Command *>&1 | Out-String -Width 240)

        if ($LASTEXITCODE -ne 0) {
            $exitCode = $LASTEXITCODE
            $status = "failed"

            if (-not $AllowFailure) {
                throw "El comando terminó con código de salida $exitCode."
            }
        }
    }
    catch {
        $status = "failed"
        $errorMessage = $_.Exception.Message

        if ([string]::IsNullOrWhiteSpace($output)) {
            $output = $_ | Out-String -Width 240
        }

        if (-not $AllowFailure) {
            $output | Out-File -FilePath $outputPath -Encoding utf8

            $metadata = [PSCustomObject]@{
                name          = $Name
                status        = $status
                started_at    = $startedAt.ToString("o")
                finished_at   = (Get-Date).ToString("o")
                duration_ms   = [math]::Round(((Get-Date) - $startedAt).TotalMilliseconds, 0)
                exit_code     = $exitCode
                allow_failure = [bool]$AllowFailure
                error         = $errorMessage
            }

            $metadata | ConvertTo-Json -Depth 10 |
                Out-File -FilePath $metadataPath -Encoding utf8

            $ErrorActionPreference = $previousErrorActionPreference
            throw
        }
    }
    finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }

    $finishedAt = Get-Date
    $output | Out-File -FilePath $outputPath -Encoding utf8

    $metadata = [PSCustomObject]@{
        name          = $Name
        status        = $status
        started_at    = $startedAt.ToString("o")
        finished_at   = $finishedAt.ToString("o")
        duration_ms   = [math]::Round(($finishedAt - $startedAt).TotalMilliseconds, 0)
        exit_code     = $exitCode
        allow_failure = [bool]$AllowFailure
        error         = $errorMessage
    }

    $metadata | ConvertTo-Json -Depth 10 |
        Out-File -FilePath $metadataPath -Encoding utf8

    Write-Host "[$status] $Name -> $outputPath"

    return $metadata
}

function Save-EvidenceEnvironment {
    param(
        [Parameter(Mandatory = $true)]
        [string]$EvidenceDirectory
    )

    Push-Location $script:RepoRoot

    try {
        Invoke-EvidenceStep -EvidenceDirectory $EvidenceDirectory -Name "git_status" -Command {
            git status --short
        } | Out-Null

        Invoke-EvidenceStep -EvidenceDirectory $EvidenceDirectory -Name "git_revision" -Command {
            git rev-parse HEAD
            git log -1 --oneline
        } | Out-Null

        Invoke-EvidenceStep -EvidenceDirectory $EvidenceDirectory -Name "go_version" -Command {
            go version
        } | Out-Null

        Invoke-EvidenceStep -EvidenceDirectory $EvidenceDirectory -Name "docker_version" -Command {
            docker version
        } -AllowFailure | Out-Null

        Invoke-EvidenceStep -EvidenceDirectory $EvidenceDirectory -Name "docker_compose_version" -Command {
            docker compose version
        } -AllowFailure | Out-Null

        Invoke-EvidenceStep -EvidenceDirectory $EvidenceDirectory -Name "system_info" -Command {
            Get-CimInstance Win32_Processor |
                Select-Object Name, NumberOfCores, NumberOfLogicalProcessors, MaxClockSpeed |
                Format-List

            Get-CimInstance Win32_ComputerSystem |
                Select-Object Manufacturer, Model, TotalPhysicalMemory |
                Format-List
        } | Out-Null
    }
    finally {
        Pop-Location
    }
}


