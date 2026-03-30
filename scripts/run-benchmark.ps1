param(
    [ValidateSet("postgres", "clickhouse", "elasticsearch")]
    [string]$Backend,

    [ValidateSet("ingest", "query", "mixed")]
    [string]$Mode,

    [int]$EPS,
    [int]$Batch,
    [int]$DurationSec,
    [int]$QueryIntervalSec,
    [int]$WorkerReadCount,

    [ValidateSet("row", "batch")]
    [string]$WriteMode,

    [string]$RunTag,

    [switch]$ResetStorage,
    [switch]$StartCollector,
    [switch]$StartWorker,
    [switch]$StartQueryRunner,
    [switch]$BuildSummary
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$commonPath = Join-Path $scriptDir "common.ps1"

if (-not (Test-Path $commonPath)) {
    throw "common.ps1 not found: $commonPath"
}

. $commonPath

function Read-MenuChoice {
    param(
        [string]$Prompt,
        [string[]]$Options,
        [int]$DefaultIndex = 1
    )

    if ($Options.Count -eq 0) {
        throw "Read-MenuChoice: options list is empty"
    }

    if ($DefaultIndex -lt 1 -or $DefaultIndex -gt $Options.Count) {
        $DefaultIndex = 1
    }

    while ($true) {
        Write-Host ""
        Write-Host $Prompt -ForegroundColor Cyan

        for ($i = 0; $i -lt $Options.Count; $i++) {
            $num = $i + 1
            $marker = if ($num -eq $DefaultIndex) { "*" } else { " " }
            Write-Host (" {0} [{1}] {2}" -f $marker, $num, $Options[$i])
        }

        $raw = Read-Host ("Enter number [default: {0}]" -f $DefaultIndex)

        if ([string]::IsNullOrWhiteSpace($raw)) {
            return $Options[$DefaultIndex - 1]
        }

        $selected = 0
        if ([int]::TryParse($raw, [ref]$selected) -and $selected -ge 1 -and $selected -le $Options.Count) {
            return $Options[$selected - 1]
        }

        Write-Host "Please enter a valid number from the list." -ForegroundColor Yellow
    }
}

function Read-IntValue {
    param(
        [string]$Prompt,
        [int]$Default,
        [int]$Min = 1
    )

    while ($true) {
        $value = Read-Host ("{0} [{1}]" -f $Prompt, $Default)

        if ([string]::IsNullOrWhiteSpace($value)) {
            return $Default
        }

        $parsed = 0
        if ([int]::TryParse($value, [ref]$parsed) -and $parsed -ge $Min) {
            return $parsed
        }

        Write-Host ("Enter an integer >= {0}" -f $Min) -ForegroundColor Yellow
    }
}

function Read-BoolValue {
    param(
        [string]$Prompt,
        [bool]$Default = $false
    )

    $defaultText = if ($Default) { "Y" } else { "N" }

    while ($true) {
        $value = Read-Host ("{0} [Y/N, default: {1}]" -f $Prompt, $defaultText)

        if ([string]::IsNullOrWhiteSpace($value)) {
            return $Default
        }

        switch ($value.ToLower()) {
            "y"   { return $true }
            "yes" { return $true }
            "n"   { return $false }
            "no"  { return $false }
            default {
                Write-Host "Enter Y or N" -ForegroundColor Yellow
            }
        }
    }
}

function Launch-CommandInNewWindow {
    param(
        [string]$CommandText
    )

    Start-Process powershell -ArgumentList @(
        "-NoExit",
        "-ExecutionPolicy", "Bypass",
        "-Command", $CommandText
    ) | Out-Null
}

function Ensure-Defaults {
    if (-not $script:Backend) { $script:Backend = "postgres" }
    if (-not $script:Mode) { $script:Mode = "ingest" }
    if (-not $script:EPS -or $script:EPS -le 0) { $script:EPS = 500 }
    if (-not $script:Batch -or $script:Batch -le 0) { $script:Batch = 10 }
    if (-not $script:DurationSec -or $script:DurationSec -le 0) { $script:DurationSec = 10 }
    if (-not $script:QueryIntervalSec -or $script:QueryIntervalSec -le 0) { $script:QueryIntervalSec = 1 }
    if (-not $script:WorkerReadCount -or $script:WorkerReadCount -le 0) { $script:WorkerReadCount = 100 }
    if (-not $script:WriteMode) { $script:WriteMode = "batch" }
    if (-not $script:RunTag) { $script:RunTag = "" }
}

function Prompt-MissingValues {
    if (-not $PSBoundParameters.ContainsKey("Backend")) {
        $backendOptions = @("postgres", "clickhouse", "elasticsearch")
        $defaultBackendIndex = [Math]::Max(1, ($backendOptions.IndexOf($script:Backend) + 1))
        $script:Backend = Read-MenuChoice -Prompt "Select backend" -Options $backendOptions -DefaultIndex $defaultBackendIndex
    }

    if (-not $PSBoundParameters.ContainsKey("Mode")) {
        $modeOptions = @("ingest", "query", "mixed")
        $defaultModeIndex = [Math]::Max(1, ($modeOptions.IndexOf($script:Mode) + 1))
        $script:Mode = Read-MenuChoice -Prompt "Select benchmark mode" -Options $modeOptions -DefaultIndex $defaultModeIndex
    }

    if ($script:Mode -eq "ingest" -or $script:Mode -eq "mixed") {
        if (-not $PSBoundParameters.ContainsKey("EPS")) {
            $script:EPS = Read-IntValue -Prompt "Enter EPS" -Default $script:EPS -Min 1
        }

        if (-not $PSBoundParameters.ContainsKey("Batch")) {
            $defaultBatch = if ($script:EPS -ge 2000) { 50 } elseif ($script:EPS -ge 1000) { 20 } else { 10 }
            $script:Batch = Read-IntValue -Prompt "Enter batch size" -Default $defaultBatch -Min 1
        }
    }

    if (-not $PSBoundParameters.ContainsKey("DurationSec")) {
        $script:DurationSec = Read-IntValue -Prompt "Enter the run duration, sec" -Default $script:DurationSec -Min 1
    }

    if (-not $PSBoundParameters.ContainsKey("WorkerReadCount")) {
        $script:WorkerReadCount = Read-IntValue -Prompt "Enter WORKER_READ_COUNT" -Default $script:WorkerReadCount -Min 1
    }

    if ($script:Backend -eq "postgres") {
        if (-not $PSBoundParameters.ContainsKey("WriteMode")) {
            $writeModeOptions = @("row", "batch")
            $defaultWriteModeIndex = [Math]::Max(1, ($writeModeOptions.IndexOf($script:WriteMode) + 1))
            $script:WriteMode = Read-MenuChoice -Prompt "Select PostgreSQL worker write mode" -Options $writeModeOptions -DefaultIndex $defaultWriteModeIndex
        }
    }
    else {
        $script:WriteMode = "batch"
    }

    if ($script:Mode -eq "query" -or $script:Mode -eq "mixed") {
        if (-not $PSBoundParameters.ContainsKey("QueryIntervalSec")) {
            $script:QueryIntervalSec = Read-IntValue -Prompt "Enter QUERY_RUNNER_INTERVAL_SEC" -Default $script:QueryIntervalSec -Min 1
        }
    }

    if (-not $PSBoundParameters.ContainsKey("RunTag")) {
        $defaultTag = "$($script:Backend)-$($script:Mode)-$($script:WriteMode)"
        if ($script:Mode -eq "ingest" -or $script:Mode -eq "mixed") {
            $defaultTag = "$defaultTag-$($script:EPS)eps"
        }

        $value = Read-Host "Enter RUN_TAG [$defaultTag]"
        if ([string]::IsNullOrWhiteSpace($value)) {
            $script:RunTag = $defaultTag
        }
        else {
            $script:RunTag = $value
        }
    }

    if (-not $PSBoundParameters.ContainsKey("ResetStorage")) {
        $script:ResetStorage = Read-BoolValue -Prompt "Reset storage data before run?" -Default $false
    }

    if (-not $PSBoundParameters.ContainsKey("StartCollector")) {
        $script:StartCollector = Read-BoolValue -Prompt "Launch collector in a new window?" -Default $true
    }

    if (-not $PSBoundParameters.ContainsKey("StartWorker")) {
        $defaultStartWorker = ($script:Mode -eq "ingest" -or $script:Mode -eq "mixed")
        $script:StartWorker = Read-BoolValue -Prompt "Launch worker in a new window?" -Default $defaultStartWorker
    }

    if (-not $PSBoundParameters.ContainsKey("StartQueryRunner")) {
        $defaultStartQuery = ($script:Mode -eq "query" -or $script:Mode -eq "mixed")
        $script:StartQueryRunner = Read-BoolValue -Prompt "Run query-runner in a new window?" -Default $defaultStartQuery
    }

    if (-not $PSBoundParameters.ContainsKey("BuildSummary")) {
        $defaultBuildSummary = ($script:Mode -eq "ingest" -or $script:Mode -eq "mixed")
        $script:BuildSummary = Read-BoolValue -Prompt "Collect ingest summary.csv after the run?" -Default $defaultBuildSummary
    }
}

function Reset-SelectedStorage {
    switch ($script:Backend) {
        "postgres"      { Reset-PostgresTable }
        "clickhouse"    { Reset-ClickHouseTable }
        "elasticsearch" { Reset-ElasticsearchIndex }
    }
}

function Start-SelectedCollector {
    $cmd = ""

    switch ($script:Backend) {
        "postgres" {
            $cmd = @"
. "$commonPath"
Start-Collector -RedisStream "events-postgres"
"@
        }
        "clickhouse" {
            $cmd = @"
. "$commonPath"
Start-Collector -RedisStream "events-clickhouse"
"@
        }
        "elasticsearch" {
            $cmd = @"
. "$commonPath"
Start-Collector -RedisStream "events-elasticsearch"
"@
        }
    }

    Launch-CommandInNewWindow -CommandText $cmd
}

function Start-SelectedWorker {
    $cmd = ""

    switch ($script:Backend) {
        "postgres" {
            $cmd = @"
. "$commonPath"
Start-WorkerPostgres -WriteMode "$($script:WriteMode)" -ReadCount $($script:WorkerReadCount)
"@
        }
        "clickhouse" {
            $cmd = @"
. "$commonPath"
Start-WorkerClickHouse -ReadCount $($script:WorkerReadCount)
"@
        }
        "elasticsearch" {
            $cmd = @"
. "$commonPath"
Start-WorkerElasticsearch -ReadCount $($script:WorkerReadCount)
"@
        }
    }

    Open-NewPowerShell $cmd
}

function Start-SelectedQueryRunner {
    $cmd = ""

    switch ($script:Backend) {
        "postgres" {
            $cmd = @"
. "$commonPath"
Start-QueryRunnerPostgres -DurationSec $($script:DurationSec) -IntervalSec $($script:QueryIntervalSec)
"@
        }
        "clickhouse" {
            $cmd = @"
. "$commonPath"
Start-QueryRunnerClickHouse -DurationSec $($script:DurationSec) -IntervalSec $($script:QueryIntervalSec)
"@
        }
        "elasticsearch" {
            $cmd = @"
. "$commonPath"
Start-QueryRunnerElasticsearch -DurationSec $($script:DurationSec) -IntervalSec $($script:QueryIntervalSec)
"@
        }
    }

    Launch-CommandInNewWindow -CommandText $cmd
}
function Run-SelectedIngest {
    switch ($script:Backend) {
        "postgres" {
            Run-IngestPostgres -EPS $script:EPS -Batch $script:Batch -DurationSec $script:DurationSec -WriteMode $script:WriteMode -RunTag $script:RunTag
        }
        "clickhouse" {
            Run-IngestClickHouse -EPS $script:EPS -Batch $script:Batch -DurationSec $script:DurationSec -RunTag $script:RunTag
        }
        "elasticsearch" {
            Run-IngestElasticsearch -EPS $script:EPS -Batch $script:Batch -DurationSec $script:DurationSec -RunTag $script:RunTag
        }
    }
}

function Run-SelectedQuery {
    Enter-RepoRoot
    try {
        switch ($script:Backend) {
            "postgres" {
                $env:QUERY_BACKEND = "postgres"
                $env:POSTGRES_DSN = "postgres://siem:siem@127.0.0.1:5432/siem?sslmode=disable"
            }
            "clickhouse" {
                $env:QUERY_BACKEND = "clickhouse"
                $env:CLICKHOUSE_DSN = "clickhouse://siem:siem@127.0.0.1:9000/siem"
            }
            "elasticsearch" {
                $env:QUERY_BACKEND = "elasticsearch"
                $env:ELASTICSEARCH_URL = "http://127.0.0.1:9200"
            }
        }

        $env:QUERY_RUNNER_DURATION_SEC = "$($script:DurationSec)"
        $env:QUERY_RUNNER_INTERVAL_SEC = "$($script:QueryIntervalSec)"
        $env:RUN_SCENARIO = "query-only"
        $env:RUN_TAG = $script:RunTag

        go run ./cmd/query-runner
    }
    finally {
        Leave-RepoRoot
    }
}

function Build-IngestSummary {
    Enter-RepoRoot
    try {
        go run ./cmd/results-aggregator
    }
    finally {
        Leave-RepoRoot
    }
}

Ensure-Defaults
Prompt-MissingValues

Write-Host ""
Write-Host "Launch configuration:" -ForegroundColor Cyan
Write-Host "  Backend:           $Backend"
Write-Host "  Mode:              $Mode"
if ($Mode -eq "ingest" -or $Mode -eq "mixed") {
    Write-Host "  EPS:               $EPS"
    Write-Host "  Batch:             $Batch"
}
Write-Host "  DurationSec:       $DurationSec"
if ($Mode -eq "query" -or $Mode -eq "mixed") {
    Write-Host "  QueryIntervalSec:  $QueryIntervalSec"
}
Write-Host "  WorkerReadCount:   $WorkerReadCount"
Write-Host "  WriteMode:         $WriteMode"
Write-Host "  RunTag:            $RunTag"
Write-Host "  ResetStorage:      $ResetStorage"
Write-Host "  StartCollector:    $StartCollector"
Write-Host "  StartWorker:       $StartWorker"
Write-Host "  StartQueryRunner:  $StartQueryRunner"
Write-Host "  BuildSummary:      $BuildSummary"
Write-Host ""

if ($ResetStorage) {
    Reset-SelectedStorage
}

if ($StartCollector) {
    Start-SelectedCollector
    Start-Sleep -Seconds 2
}

if ($StartWorker -and ($Mode -eq "ingest" -or $Mode -eq "mixed")) {
    Start-SelectedWorker
    Start-Sleep -Seconds 3
}

switch ($Mode) {
    "ingest" {
        Run-SelectedIngest
    }
    "query" {
        if ($StartQueryRunner) {
            Start-SelectedQueryRunner
            Write-Host "Query-runner launched in a new window." -ForegroundColor Green
        }
        else {
            Run-SelectedQuery
        }
    }
    "mixed" {
        if ($StartQueryRunner) {
            Start-SelectedQueryRunner
            Start-Sleep -Seconds 2
        }
        Run-SelectedIngest
    }
}

if ($BuildSummary -and ($Mode -eq "ingest" -or $Mode -eq "mixed")) {
    Write-Host "I'm collecting ingest summary..." -ForegroundColor Cyan
    Build-IngestSummary
}

Write-Host "The scenario is complete." -ForegroundColor Green