param(
    [ValidateSet("postgres", "clickhouse", "elasticsearch", "cassandra")]
    [string]$Backend = "postgres",

    [ValidateSet("ingest", "query", "ingest-only", "query-only", "mixed", "longrun-ingest", "longrun-mixed")]
    [string]$Mode = "ingest-only",

    [int]$EPS = 500,
    [int]$Batch = 10,
    [int]$DurationSec = 10,
    [int]$QueryIntervalSec = 1,
    [int]$QueryWarmupSec = 3,
    [int]$QueryConcurrency = 1,
    [string]$WorkloadPath = "scenarios/query-default.json",
    [int]$WorkerReadCount = 100,

    [ValidateSet("row", "batch")]
    [string]$WriteMode = "batch",

    [string]$RunTag = "",

    [switch]$ResetStorage,
    [switch]$StartCollector,
    [switch]$StartWorker,
    [switch]$StartQueryRunner,
    [switch]$BuildSummary
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$commonPath = Join-Path $scriptDir "common.ps1"
if (-not (Test-Path $commonPath)) { throw "common.ps1 not found: $commonPath" }
. $commonPath

function Normalize-Mode {
    param([string]$Value)
    switch ($Value) {
        "ingest" { return "ingest-only" }
        "query" { return "query-only" }
        default { return $Value }
    }
}

$Scenario = Normalize-Mode $Mode
$IngestModes = @("ingest-only", "mixed", "longrun-ingest", "longrun-mixed")
$QueryModes = @("query-only", "mixed", "longrun-mixed")
$IsIngestMode = $IngestModes -contains $Scenario
$IsQueryMode = $QueryModes -contains $Scenario

if (($Scenario -eq "longrun-ingest" -or $Scenario -eq "longrun-mixed") -and $DurationSec -eq 10) {
    $DurationSec = 1800
}

if ($Backend -ne "postgres") { $WriteMode = "batch" }
if ([string]::IsNullOrWhiteSpace($RunTag)) {
    $RunTag = "$Backend-$Scenario-$WriteMode"
    if ($IsIngestMode) { $RunTag = "$RunTag-${EPS}eps" }
}

function Get-RedisStreamForBackend {
    param([string]$Name)
    switch ($Name) {
        "postgres" { return "events-postgres" }
        "clickhouse" { return "events-clickhouse" }
        "elasticsearch" { return "events-elasticsearch" }
        "cassandra" { return "events-cassandra" }
        default { throw "Unsupported backend: $Name" }
    }
}

function Reset-SelectedStorage {
    switch ($Backend) {
        "postgres" { Reset-PostgresTable }
        "clickhouse" { Reset-ClickHouseTable }
        "elasticsearch" { Reset-ElasticsearchIndex }
        "cassandra" { Reset-CassandraTable }
        default { throw "Unsupported backend for reset: $Backend" }
    }
    Reset-RedisForBackend -Backend $Backend
}

function Start-SelectedCollector {
    $stream = Get-RedisStreamForBackend $Backend
    Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream '$stream'" | Out-Null
}

function Start-SelectedWorker {
    switch ($Backend) {
        "postgres" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerPostgres -WriteMode '$WriteMode' -ReadCount $WorkerReadCount -Scenario '$Scenario'" | Out-Null
        }
        "clickhouse" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerClickHouse -ReadCount $WorkerReadCount -Scenario '$Scenario'" | Out-Null
        }
        "elasticsearch" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerElasticsearch -ReadCount $WorkerReadCount -Scenario '$Scenario'" | Out-Null
        }
        "cassandra" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerCassandra -ReadCount $WorkerReadCount -Scenario '$Scenario'" | Out-Null
        }
        default { throw "Unsupported backend for worker: $Backend" }
    }
}

function Start-SelectedQueryRunner {
    switch ($Backend) {
        "postgres" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerPostgres -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$WorkloadPath' -Scenario '$Scenario' -RunTag '$RunTag'" | Out-Null
        }
        "clickhouse" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerClickHouse -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$WorkloadPath' -Scenario '$Scenario' -RunTag '$RunTag'" | Out-Null
        }
        "elasticsearch" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerElasticsearch -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$WorkloadPath' -Scenario '$Scenario' -RunTag '$RunTag'" | Out-Null
        }
        "cassandra" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerCassandra -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$WorkloadPath' -Scenario '$Scenario' -RunTag '$RunTag'" | Out-Null
        }
        default { throw "Unsupported backend for query-runner: $Backend" }
    }
}

function Run-SelectedQuery {
    switch ($Backend) {
        "postgres" { Start-QueryRunnerPostgres -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
        "clickhouse" { Start-QueryRunnerClickHouse -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
        "elasticsearch" { Start-QueryRunnerElasticsearch -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
        "cassandra" { Start-QueryRunnerCassandra -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
        default { throw "Unsupported backend for query run: $Backend" }
    }
}

function Run-SelectedIngest {
    switch ($Backend) {
        "postgres" { Run-IngestPostgres -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode $WriteMode -RunTag $RunTag -Scenario $Scenario }
        "clickhouse" { Run-IngestClickHouse -EPS $EPS -Batch $Batch -DurationSec $DurationSec -RunTag $RunTag -Scenario $Scenario }
        "elasticsearch" { Run-IngestElasticsearch -EPS $EPS -Batch $Batch -DurationSec $DurationSec -RunTag $RunTag -Scenario $Scenario }
        "cassandra" { Run-IngestCassandra -EPS $EPS -Batch $Batch -DurationSec $DurationSec -RunTag $RunTag -Scenario $Scenario }
        default { throw "Unsupported backend for ingest: $Backend" }
    }
}

function Get-IngestSummaryPaths {
    switch ($Scenario) {
        "mixed" { return @{ Glob = "results/mixed/ingest-*.json"; Output = "results/mixed/summary-ingest.csv" } }
        "longrun-ingest" { return @{ Glob = "results/longrun-ingest/ingest-*.json"; Output = "results/longrun-ingest/summary.csv" } }
        "longrun-mixed" { return @{ Glob = "results/longrun-mixed/ingest-*.json"; Output = "results/longrun-mixed/summary-ingest.csv" } }
        default { return @{ Glob = "results/ingest/*.json"; Output = "results/ingest/summary.csv" } }
    }
}

function Get-QuerySummaryPaths {
    switch ($Scenario) {
        "mixed" { return @{ Glob = "results/mixed/query-*.json"; Output = "results/mixed/summary-query.csv" } }
        "longrun-mixed" { return @{ Glob = "results/longrun-mixed/query-*.json"; Output = "results/longrun-mixed/summary-query.csv" } }
        default { return @{ Glob = "results/query/query-*.json"; Output = "results/query/summary.csv" } }
    }
}

function Build-IngestSummaryForScenario {
    $paths = Get-IngestSummaryPaths
    Enter-RepoRoot
    try {
        $env:RESULTS_GLOB = $paths.Glob
        $env:RESULTS_OUTPUT = $paths.Output
        go run ./cmd/results-aggregator
    }
    finally {
        Remove-Item Env:RESULTS_GLOB -ErrorAction SilentlyContinue
        Remove-Item Env:RESULTS_OUTPUT -ErrorAction SilentlyContinue
        Leave-RepoRoot
    }
}

function Build-QuerySummaryForScenario {
    $paths = Get-QuerySummaryPaths
    Enter-RepoRoot
    try {
        $env:RESULTS_GLOB = $paths.Glob
        $env:RESULTS_OUTPUT = $paths.Output
        go run ./cmd/query-results-aggregator
    }
    finally {
        Remove-Item Env:RESULTS_GLOB -ErrorAction SilentlyContinue
        Remove-Item Env:RESULTS_OUTPUT -ErrorAction SilentlyContinue
        Leave-RepoRoot
    }
}

Write-Host ""
Write-Host "Launch configuration:" -ForegroundColor Cyan
Write-Host "  Backend:           $Backend"
Write-Host "  Mode:              $Mode"
Write-Host "  Scenario:          $Scenario"
if ($IsIngestMode) {
    Write-Host "  EPS:               $EPS"
    Write-Host "  Batch:             $Batch"
}
Write-Host "  DurationSec:       $DurationSec"
if ($IsQueryMode) {
    Write-Host "  QueryIntervalSec:  $QueryIntervalSec"
    Write-Host "  QueryWarmupSec:    $QueryWarmupSec"
    Write-Host "  QueryConcurrency:  $QueryConcurrency"
    Write-Host "  WorkloadPath:      $WorkloadPath"
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

if ($ResetStorage) { Reset-SelectedStorage }

if ($StartCollector) {
    Start-SelectedCollector
    Start-Sleep -Seconds 2
}

if ($StartWorker -and $IsIngestMode) {
    Start-SelectedWorker
    Start-Sleep -Seconds 3
}

if ($IsQueryMode) {
    if ($StartQueryRunner) {
        Start-SelectedQueryRunner
        Start-Sleep -Seconds 2
    }
    elseif (-not $IsIngestMode) {
        Run-SelectedQuery
    }
}

if ($IsIngestMode) { Run-SelectedIngest }

if ($IsQueryMode -and $StartQueryRunner -and ($Scenario -eq "mixed" -or $Scenario -eq "longrun-mixed")) {
    Start-Sleep -Seconds ($QueryWarmupSec + $DurationSec + 2)
}

if ($BuildSummary) {
    if ($IsIngestMode) { Build-IngestSummaryForScenario }
    if ($IsQueryMode) { Build-QuerySummaryForScenario }
}

Write-Host "The scenario is complete." -ForegroundColor Green
