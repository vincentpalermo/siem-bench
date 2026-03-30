param(
    [string]$MatrixPath = ".\scripts\matrix-default.json",
    [switch]$StopGoServicesBeforeStart,
    [switch]$StartInfra
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")
$commonPath = Join-Path $scriptDir "common.ps1"
$stopGoPath = Join-Path $scriptDir "stop-go-services.ps1"
$startInfraPath = Join-Path $scriptDir "start-infra.ps1"

if (-not (Test-Path $commonPath)) {
    throw "common.ps1 not found: $commonPath"
}

if (-not (Test-Path $MatrixPath)) {
    throw "Matrix file not found: $MatrixPath"
}

. $commonPath

function Get-JsonFile {
    param(
        [string]$Path
    )

    Get-Content $Path -Raw | ConvertFrom-Json
}

function Ensure-Array {
    param(
        $Value,
        $DefaultValue
    )

    if ($null -eq $Value) {
        return @($DefaultValue)
    }

    if ($Value -is [System.Array]) {
        return @($Value)
    }

    return @($Value)
}

function Stop-GoServices {
    if (Test-Path $stopGoPath) {
        Write-Host "Stopping running Go services..." -ForegroundColor Yellow
        powershell -ExecutionPolicy Bypass -File $stopGoPath
    }
}

function Start-Infrastructure {
    if (Test-Path $startInfraPath) {
        Write-Host "Starting Docker infrastructure..." -ForegroundColor Cyan
        powershell -ExecutionPolicy Bypass -File $startInfraPath
    }
}

function Reset-StorageForBackend {
    param(
        [string]$Backend
    )

    switch ($Backend) {
        "postgres" {
            Write-Host "Resetting PostgreSQL storage..." -ForegroundColor Cyan
            Reset-PostgresTable
        }
        "clickhouse" {
            Write-Host "Resetting ClickHouse storage..." -ForegroundColor Cyan
            Reset-ClickHouseTable
        }
        "elasticsearch" {
            Write-Host "Resetting Elasticsearch storage..." -ForegroundColor Cyan
            Reset-ElasticsearchIndex
        }
        default {
            throw "Unsupported backend for reset: $Backend"
        }
    }
}

function Start-CollectorForBackend {
    param(
        [string]$Backend
    )

    switch ($Backend) {
        "postgres" {
            Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-postgres'"
        }
        "clickhouse" {
            Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-clickhouse'"
        }
        "elasticsearch" {
            Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-elasticsearch'"
        }
        default {
            throw "Unsupported backend for collector: $Backend"
        }
    }
}

function Start-WorkerForBackend {
    param(
        [string]$Backend,
        [string]$WriteMode,
        [int]$ReadCount
    )

    switch ($Backend) {
        "postgres" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerPostgres -WriteMode '$WriteMode' -ReadCount $ReadCount"
        }
        "clickhouse" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerClickHouse -ReadCount $ReadCount"
        }
        "elasticsearch" {
            Open-NewPowerShell ". '$commonPath'; Start-WorkerElasticsearch -ReadCount $ReadCount"
        }
        default {
            throw "Unsupported backend for worker: $Backend"
        }
    }
}

function Start-QueryRunnerWindow {
    param(
        [string]$Backend,
        [int]$DurationSec,
        [int]$IntervalSec
    )

    switch ($Backend) {
        "postgres" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerPostgres -DurationSec $DurationSec -IntervalSec $IntervalSec"
        }
        "clickhouse" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerClickHouse -DurationSec $DurationSec -IntervalSec $IntervalSec"
        }
        "elasticsearch" {
            Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerElasticsearch -DurationSec $DurationSec -IntervalSec $IntervalSec"
        }
        default {
            throw "Unsupported backend for query-runner: $Backend"
        }
    }
}

function Invoke-QueryRunnerDirect {
    param(
        [string]$Backend,
        [int]$DurationSec,
        [int]$IntervalSec,
        [int]$WarmupSec,
        [int]$Concurrency,
        [string]$WorkloadPath,
        [string]$RunTag,
        [string]$Scenario = "query-only"
    )

    Enter-RepoRoot
    try {
        $env:QUERY_BACKEND = $Backend
        $env:QUERY_RUNNER_DURATION_SEC = "$DurationSec"
        $env:QUERY_RUNNER_INTERVAL_SEC = "$IntervalSec"
        $env:QUERY_RUNNER_WARMUP_SEC = "$WarmupSec"
        $env:QUERY_RUNNER_CONCURRENCY = "$Concurrency"
        $env:QUERY_WORKLOAD_PATH = $WorkloadPath
        $env:RUN_SCENARIO = $Scenario
        $env:RUN_TAG = $RunTag

        switch ($Backend) {
            "postgres" {
                $env:POSTGRES_DSN = "postgres://siem:siem@127.0.0.1:5432/siem?sslmode=disable"
            }
            "clickhouse" {
                $env:CLICKHOUSE_DSN = "clickhouse://siem:siem@127.0.0.1:9000/siem"
            }
            "elasticsearch" {
                $env:ELASTICSEARCH_URL = "http://127.0.0.1:9200"
            }
        }

        go run ./cmd/query-runner
    }
    finally {
        Leave-RepoRoot
    }
}

function Invoke-IngestDirect {
    param(
        [string]$Backend,
        [int]$EPS,
        [int]$Batch,
        [int]$DurationSec,
        [string]$WriteMode,
        [string]$RunTag,
        [string]$Scenario = "ingest-only"
    )

    switch ($Backend) {
        "postgres" {
            Run-IngestPostgres -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode $WriteMode -RunTag $RunTag -Scenario $Scenario
        }
        "clickhouse" {
            Run-IngestClickHouse -EPS $EPS -Batch $Batch -DurationSec $DurationSec -RunTag $RunTag -Scenario $Scenario
        }
        "elasticsearch" {
            Run-IngestElasticsearch -EPS $EPS -Batch $Batch -DurationSec $DurationSec -RunTag $RunTag -Scenario $Scenario
        }
        default {
            throw "Unsupported backend for ingest: $Backend"
        }
    }
}

function Build-IngestSummary {
    param(
        [string]$ResultsGlob = "results/ingest/*.json",
        [string]$OutputPath = "results/ingest/summary.csv"
    )

    Enter-RepoRoot
    try {
        $env:RESULTS_GLOB = $ResultsGlob
        $env:RESULTS_OUTPUT = $OutputPath
        go run ./cmd/results-aggregator
    }
    finally {
        Remove-Item Env:\RESULTS_GLOB -ErrorAction SilentlyContinue
        Remove-Item Env:\RESULTS_OUTPUT -ErrorAction SilentlyContinue
        Leave-RepoRoot
    }
}

function Build-QuerySummary {
    param(
        [string]$ResultsGlob = "results/query/query-*.json",
        [string]$OutputPath = "results/query/summary.csv"
    )

    Enter-RepoRoot
    try {
        $env:RESULTS_GLOB = $ResultsGlob
        $env:RESULTS_OUTPUT = $OutputPath
        go run ./cmd/query-results-aggregator
    }
    finally {
        Remove-Item Env:\RESULTS_GLOB -ErrorAction SilentlyContinue
        Remove-Item Env:\RESULTS_OUTPUT -ErrorAction SilentlyContinue
        Leave-RepoRoot
    }
}

function Invoke-MatrixRun {
    param(
        [string]$Backend,
        [string]$Mode,
        [int]$EPS,
        [int]$Batch,
        [int]$DurationSec,
        [string]$WriteMode,
        [int]$QueryIntervalSec,
        [int]$QueryWarmupSec,
        [int]$QueryConcurrency,
        [string]$QueryWorkloadPath,
        [int]$WorkerReadCount,
        [int]$SleepAfterStartSec,
        [int]$SleepAfterStopSec,
        [bool]$ResetStorageEachRun,
        [bool]$BuildSummary,
        [int]$RepeatIndex
    )

    $runTag = "$Backend-$Mode"
    if ($Mode -eq "ingest" -or $Mode -eq "mixed") {
        $runTag += "-$EPSeps-$WriteMode"
    }
    if ($Mode -eq "query" -or $Mode -eq "mixed") {
        $runTag += "-q$QueryConcurrency"
    }
    $runTag += "-r$RepeatIndex"

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "Starting matrix run: $runTag" -ForegroundColor Cyan
    Write-Host "Backend=$Backend Mode=$Mode EPS=$EPS Batch=$Batch Duration=$DurationSec WriteMode=$WriteMode QueryConcurrency=$QueryConcurrency Repeat=$RepeatIndex"
    Write-Host "========================================" -ForegroundColor Cyan

    Stop-GoServices
    Start-Sleep -Seconds $SleepAfterStopSec

    if ($ResetStorageEachRun) {
        Reset-StorageForBackend -Backend $Backend
    }

    if ($Mode -eq "ingest" -or $Mode -eq "mixed") {
        Start-CollectorForBackend -Backend $Backend
        Start-Sleep -Seconds 2

        Start-WorkerForBackend -Backend $Backend -WriteMode $WriteMode -ReadCount $WorkerReadCount
        Start-Sleep -Seconds $SleepAfterStartSec
    }

    switch ($Mode) {
        "ingest" {
            Invoke-IngestDirect -Backend $Backend -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode $WriteMode -RunTag $runTag -Scenario "ingest-only"

            if ($BuildSummary) {
                Build-IngestSummary -ResultsGlob "results/ingest/*.json" -OutputPath "results/ingest/summary.csv"
            }
        }

        "query" {
            Invoke-QueryRunnerDirect `
                -Backend $Backend `
                -DurationSec $DurationSec `
                -IntervalSec $QueryIntervalSec `
                -WarmupSec $QueryWarmupSec `
                -Concurrency $QueryConcurrency `
                -WorkloadPath $QueryWorkloadPath `
                -RunTag $runTag `
                -Scenario "query-only"

            if ($BuildSummary) {
                Build-QuerySummary -ResultsGlob "results/query/query-*.json" -OutputPath "results/query/summary.csv"
            }
        }

        "mixed" {
            switch ($Backend) {
                "postgres" {
                    Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerPostgres -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$QueryWorkloadPath' -Scenario 'mixed' -RunTag '$runTag'"
                }
                "clickhouse" {
                    Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerClickHouse -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$QueryWorkloadPath' -Scenario 'mixed' -RunTag '$runTag'"
                }
                "elasticsearch" {
                    Open-NewPowerShell ". '$commonPath'; Start-QueryRunnerElasticsearch -DurationSec $DurationSec -IntervalSec $QueryIntervalSec -WarmupSec $QueryWarmupSec -Concurrency $QueryConcurrency -WorkloadPath '$QueryWorkloadPath' -Scenario 'mixed' -RunTag '$runTag'"
                }
                default {
                    throw "Unsupported backend for mixed query-runner: $Backend"
                }
            }

            Start-Sleep -Seconds 2

            Invoke-IngestDirect -Backend $Backend -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode $WriteMode -RunTag $runTag -Scenario "mixed"

            Start-Sleep -Seconds ($QueryWarmupSec + $DurationSec + 2)

            if ($BuildSummary) {
                Build-IngestSummary -ResultsGlob "results/mixed/ingest-*.json" -OutputPath "results/mixed/summary-ingest.csv"
                Build-QuerySummary -ResultsGlob "results/mixed/query-*.json" -OutputPath "results/mixed/summary-query.csv"
            }
        }

        default {
            throw "Unsupported mode: $Mode"
        }
    }

    Write-Host "Finished matrix run: $runTag" -ForegroundColor Green
}

function Expand-And-RunMatrix {
    param(
        $Matrix
    )

    $defaults = $Matrix.defaults

    foreach ($run in $Matrix.runs) {
        $backend = $run.backend
        $mode = $run.mode

        $durationSec = if ($null -ne $run.duration_sec) { [int]$run.duration_sec } else { [int]$defaults.duration_sec }
        $queryIntervalSec = if ($null -ne $run.query_interval_sec) { [int]$run.query_interval_sec } else { [int]$defaults.query_interval_sec }
        $queryWarmupSec = if ($null -ne $run.query_warmup_sec) { [int]$run.query_warmup_sec } else { [int]$defaults.query_warmup_sec }
        $workerReadCount = if ($null -ne $run.worker_read_count) { [int]$run.worker_read_count } else { [int]$defaults.worker_read_count }
        $batch = if ($null -ne $run.batch) { [int]$run.batch } else { [int]$defaults.batch }
        $repeats = if ($null -ne $run.repeats) { [int]$run.repeats } else { [int]$defaults.repeats }
        $sleepAfterStartSec = if ($null -ne $run.sleep_after_start_sec) { [int]$run.sleep_after_start_sec } else { [int]$defaults.sleep_after_start_sec }
        $sleepAfterStopSec = if ($null -ne $run.sleep_after_stop_sec) { [int]$run.sleep_after_stop_sec } else { [int]$defaults.sleep_after_stop_sec }
        $buildSummary = if ($null -ne $run.build_summary) { [bool]$run.build_summary } else { [bool]$defaults.build_summary }
        $resetStorageEachRun = if ($null -ne $run.reset_storage_each_run) { [bool]$run.reset_storage_each_run } else { [bool]$defaults.reset_storage_each_run }

        $epsValues = Ensure-Array -Value $run.eps -DefaultValue 0
        $writeModes = Ensure-Array -Value $run.write_modes -DefaultValue $defaults.write_mode
        $queryConcurrencyValues = Ensure-Array -Value $run.query_concurrency -DefaultValue $defaults.query_concurrency

        $queryWorkloadPath = if ($null -ne $run.query_workload_path) { [string]$run.query_workload_path } else { "scenarios/query-default.json" }

        for ($repeat = 1; $repeat -le $repeats; $repeat++) {
            foreach ($writeMode in $writeModes) {
                foreach ($qConcurrency in $queryConcurrencyValues) {
                    if ($mode -eq "query") {
                        Invoke-MatrixRun `
                            -Backend $backend `
                            -Mode $mode `
                            -EPS 0 `
                            -Batch $batch `
                            -DurationSec $durationSec `
                            -WriteMode $writeMode `
                            -QueryIntervalSec $queryIntervalSec `
                            -QueryWarmupSec $queryWarmupSec `
                            -QueryConcurrency ([int]$qConcurrency) `
                            -QueryWorkloadPath $queryWorkloadPath `
                            -WorkerReadCount $workerReadCount `
                            -SleepAfterStartSec $sleepAfterStartSec `
                            -SleepAfterStopSec $sleepAfterStopSec `
                            -ResetStorageEachRun $resetStorageEachRun `
                            -BuildSummary $buildSummary `
                            -RepeatIndex $repeat
                    }
                    else {
                        foreach ($eps in $epsValues) {
                            Invoke-MatrixRun `
                                -Backend $backend `
                                -Mode $mode `
                                -EPS ([int]$eps) `
                                -Batch $batch `
                                -DurationSec $durationSec `
                                -WriteMode ([string]$writeMode) `
                                -QueryIntervalSec $queryIntervalSec `
                                -QueryWarmupSec $queryWarmupSec `
                                -QueryConcurrency ([int]$qConcurrency) `
                                -QueryWorkloadPath $queryWorkloadPath `
                                -WorkerReadCount $workerReadCount `
                                -SleepAfterStartSec $sleepAfterStartSec `
                                -SleepAfterStopSec $sleepAfterStopSec `
                                -ResetStorageEachRun $resetStorageEachRun `
                                -BuildSummary $buildSummary `
                                -RepeatIndex $repeat
                        }
                    }
                }
            }
        }
    }
}

Write-Host "Loading matrix from: $MatrixPath" -ForegroundColor Cyan
$matrix = Get-JsonFile -Path $MatrixPath

if ($StartInfra) {
    Start-Infrastructure
}

if ($StopGoServicesBeforeStart) {
    Stop-GoServices
}

Expand-And-RunMatrix -Matrix $matrix

Write-Host ""
Write-Host "Matrix execution completed." -ForegroundColor Green