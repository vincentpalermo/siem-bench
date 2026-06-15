$ErrorActionPreference = "Stop"

$Script:ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Script:RepoRoot = Resolve-Path (Join-Path $Script:ScriptDir "..")

function Enter-RepoRoot { Push-Location $Script:RepoRoot }
function Leave-RepoRoot { Pop-Location }

function Open-NewPowerShell {
    param([string]$Command)
    return Start-Process powershell -ArgumentList @("-NoExit", "-ExecutionPolicy", "Bypass", "-Command", $Command) -PassThru
}

function Stop-ProcessTreeSafe {
    param([System.Diagnostics.Process]$Process)
    if ($null -eq $Process) { return }
    try {
        if (-not $Process.HasExited) {
            taskkill /PID $Process.Id /T /F | Out-Null
            Write-Host "Stopped process tree PID $($Process.Id)" -ForegroundColor Green
        }
    }
    catch {
        Write-Host "Failed to stop process tree PID $($Process.Id): $($_.Exception.Message)" -ForegroundColor Yellow
    }
}

function Set-CommonEnv {
    param(
        [string]$RedisAddr = "127.0.0.1:6379",
        [string]$RedisStream,
        [string]$RedisGroup = "",
        [string]$RedisConsumer = ""
    )
    $env:REDIS_ADDR = $RedisAddr
    $env:REDIS_STREAM = $RedisStream
    if ($RedisGroup -ne "") { $env:REDIS_GROUP = $RedisGroup }
    if ($RedisConsumer -ne "") { $env:REDIS_CONSUMER = $RedisConsumer }
}

function Set-BackendEnv {
    param([string]$Backend)
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
        "cassandra" {
            $env:CASSANDRA_HOSTS = "127.0.0.1"
            $env:CASSANDRA_PORT = "9042"
            $env:CASSANDRA_KEYSPACE = "siem"
            $env:CASSANDRA_CONSISTENCY = "one"
        }
        default { throw "Unsupported backend env: $Backend" }
    }
}

function Reset-RedisStream {
    param([string]$StreamName)
    if ([string]::IsNullOrWhiteSpace($StreamName)) { throw "Reset-RedisStream: StreamName is empty" }
    docker exec -i siem-redis redis-cli DEL $StreamName | Out-Null
    Write-Host "Redis stream reset: $StreamName" -ForegroundColor Cyan
}

function Reset-RedisForBackend {
    param([string]$Backend)
    switch ($Backend) {
        "postgres" { Reset-RedisStream -StreamName "events-postgres" }
        "clickhouse" { Reset-RedisStream -StreamName "events-clickhouse" }
        "elasticsearch" { Reset-RedisStream -StreamName "events-elasticsearch" }
        "cassandra" { Reset-RedisStream -StreamName "events-cassandra" }
        default { throw "Unsupported backend for Redis reset: $Backend" }
    }
}

function Reset-PostgresTable {
    $ddlPath = Join-Path $Script:RepoRoot "migrations\001_init.sql"
    if (-not (Test-Path $ddlPath)) { throw "PostgreSQL DDL file not found: $ddlPath" }
    Get-Content $ddlPath -Raw | docker exec -i siem-postgres psql -U siem -d siem | Out-Null
    docker exec -i siem-postgres psql -U siem -d siem -c "TRUNCATE TABLE events RESTART IDENTITY;" | Out-Null
    Write-Host "PostgreSQL table reset: events" -ForegroundColor Cyan
}

function Reset-ClickHouseTable {
    $ddlPath = Join-Path $Script:RepoRoot "migrations\clickhouse\001_init.sql"
    if (-not (Test-Path $ddlPath)) { throw "ClickHouse DDL file not found: $ddlPath" }
    docker exec -i siem-clickhouse clickhouse-client --multiquery --query "CREATE DATABASE IF NOT EXISTS siem; TRUNCATE TABLE IF EXISTS siem.events;" | Out-Null
    Get-Content $ddlPath -Raw | docker exec -i siem-clickhouse clickhouse-client --multiquery | Out-Null
    Write-Host "ClickHouse table reset: siem.events" -ForegroundColor Cyan
}

function Reset-ElasticsearchIndex {
    $mappingPath = Join-Path $Script:RepoRoot "deploy\elasticsearch\index-template.json"
    if (-not (Test-Path $mappingPath)) { throw "Elasticsearch mapping file not found: $mappingPath" }
    try { Invoke-RestMethod -Method DELETE -Uri "http://127.0.0.1:9200/siem-events" | Out-Null } catch { Write-Host "Elasticsearch index delete skipped or failed; continuing..." -ForegroundColor Yellow }
    $body = Get-Content $mappingPath -Raw
    Invoke-RestMethod -Method PUT -Uri "http://127.0.0.1:9200/siem-events" -ContentType "application/json" -Body $body | Out-Null
    Write-Host "Elasticsearch index reset: siem-events" -ForegroundColor Cyan
}

function Reset-CassandraTable {
    $ddlPath = Join-Path $Script:RepoRoot "deploy\cassandra\init.cql"
    if (-not (Test-Path $ddlPath)) { throw "Cassandra CQL file not found: $ddlPath" }
    Get-Content $ddlPath -Raw | docker exec -i siem-cassandra cqlsh | Out-Null
    try { docker exec -i siem-cassandra cqlsh -e "TRUNCATE siem.events;" | Out-Null } catch { Write-Host "Cassandra truncate skipped or failed; continuing..." -ForegroundColor Yellow }
    Write-Host "Cassandra table reset: siem.events" -ForegroundColor Cyan
}

function Start-Collector {
    param([string]$RedisStream, [string]$HttpAddr = ":8080")
    Enter-RepoRoot
    try {
        Set-CommonEnv -RedisStream $RedisStream
        $env:HTTP_ADDR = $HttpAddr
        go run ./cmd/collector
    }
    finally { Leave-RepoRoot }
}

function Start-WorkerPostgres {
    param([string]$WriteMode = "batch", [int]$ReadCount = 100, [string]$Scenario = "ingest-only")
    Enter-RepoRoot
    try {
        Set-CommonEnv -RedisStream "events-postgres" -RedisGroup "workers-postgres" -RedisConsumer "worker-postgres-1"
        Set-BackendEnv -Backend "postgres"
        $env:WORKER_READ_COUNT = "$ReadCount"
        $env:WORKER_WRITE_MODE = $WriteMode
        $env:RUN_SCENARIO = $Scenario
        go run ./cmd/worker-postgres
    }
    finally { Leave-RepoRoot }
}

function Start-WorkerClickHouse {
    param([int]$ReadCount = 100, [string]$Scenario = "ingest-only")
    Enter-RepoRoot
    try {
        Set-CommonEnv -RedisStream "events-clickhouse" -RedisGroup "workers-clickhouse" -RedisConsumer "worker-clickhouse-1"
        Set-BackendEnv -Backend "clickhouse"
        $env:WORKER_READ_COUNT = "$ReadCount"
        $env:WORKER_WRITE_MODE = "batch"
        $env:RUN_SCENARIO = $Scenario
        go run ./cmd/worker-clickhouse
    }
    finally { Leave-RepoRoot }
}

function Start-WorkerElasticsearch {
    param([int]$ReadCount = 100, [string]$Scenario = "ingest-only")
    Enter-RepoRoot
    try {
        Set-CommonEnv -RedisStream "events-elasticsearch" -RedisGroup "workers-elasticsearch" -RedisConsumer "worker-elasticsearch-1"
        Set-BackendEnv -Backend "elasticsearch"
        $env:WORKER_READ_COUNT = "$ReadCount"
        $env:WORKER_WRITE_MODE = "batch"
        $env:RUN_SCENARIO = $Scenario
        go run ./cmd/worker-elasticsearch
    }
    finally { Leave-RepoRoot }
}

function Start-WorkerCassandra {
    param([int]$ReadCount = 100, [string]$Scenario = "ingest-only")
    Enter-RepoRoot
    try {
        Set-CommonEnv -RedisStream "events-cassandra" -RedisGroup "workers-cassandra" -RedisConsumer "worker-cassandra-1"
        Set-BackendEnv -Backend "cassandra"
        $env:WORKER_READ_COUNT = "$ReadCount"
        $env:WORKER_WRITE_MODE = "batch"
        $env:RUN_SCENARIO = $Scenario
        go run ./cmd/worker-cassandra
    }
    finally { Leave-RepoRoot }
}

function Start-QueryRunnerGeneric {
    param(
        [string]$Backend,
        [int]$DurationSec = 10,
        [int]$IntervalSec = 1,
        [int]$WarmupSec = 3,
        [int]$Concurrency = 1,
        [string]$WorkloadPath = "scenarios/query-default.json",
        [string]$Scenario = "query-only",
        [string]$RunTag = ""
    )
    Enter-RepoRoot
    try {
        $env:QUERY_BACKEND = $Backend
        Set-BackendEnv -Backend $Backend
        $env:QUERY_RUNNER_DURATION_SEC = "$DurationSec"
        $env:QUERY_RUNNER_INTERVAL_SEC = "$IntervalSec"
        $env:QUERY_RUNNER_WARMUP_SEC = "$WarmupSec"
        $env:QUERY_RUNNER_CONCURRENCY = "$Concurrency"
        $env:QUERY_WORKLOAD_PATH = $WorkloadPath
        $env:RUN_SCENARIO = $Scenario
        $env:RUN_TAG = $RunTag
        go run ./cmd/query-runner
    }
    finally { Leave-RepoRoot }
}

function Start-QueryRunnerPostgres { param([int]$DurationSec = 10,[int]$IntervalSec = 1,[int]$WarmupSec = 3,[int]$Concurrency = 1,[string]$WorkloadPath = "scenarios/query-default.json",[string]$Scenario = "query-only",[string]$RunTag = ""); Start-QueryRunnerGeneric -Backend "postgres" -DurationSec $DurationSec -IntervalSec $IntervalSec -WarmupSec $WarmupSec -Concurrency $Concurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
function Start-QueryRunnerClickHouse { param([int]$DurationSec = 10,[int]$IntervalSec = 1,[int]$WarmupSec = 3,[int]$Concurrency = 1,[string]$WorkloadPath = "scenarios/query-default.json",[string]$Scenario = "query-only",[string]$RunTag = ""); Start-QueryRunnerGeneric -Backend "clickhouse" -DurationSec $DurationSec -IntervalSec $IntervalSec -WarmupSec $WarmupSec -Concurrency $Concurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
function Start-QueryRunnerElasticsearch { param([int]$DurationSec = 10,[int]$IntervalSec = 1,[int]$WarmupSec = 3,[int]$Concurrency = 1,[string]$WorkloadPath = "scenarios/query-default.json",[string]$Scenario = "query-only",[string]$RunTag = ""); Start-QueryRunnerGeneric -Backend "elasticsearch" -DurationSec $DurationSec -IntervalSec $IntervalSec -WarmupSec $WarmupSec -Concurrency $Concurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }
function Start-QueryRunnerCassandra { param([int]$DurationSec = 10,[int]$IntervalSec = 1,[int]$WarmupSec = 3,[int]$Concurrency = 1,[string]$WorkloadPath = "scenarios/query-default.json",[string]$Scenario = "query-only",[string]$RunTag = ""); Start-QueryRunnerGeneric -Backend "cassandra" -DurationSec $DurationSec -IntervalSec $IntervalSec -WarmupSec $WarmupSec -Concurrency $Concurrency -WorkloadPath $WorkloadPath -Scenario $Scenario -RunTag $RunTag }

function Run-IngestGeneric {
    param([string]$Backend,[int]$EPS,[int]$Batch,[int]$DurationSec = 10,[string]$WriteMode = "batch",[string]$RunTag = "",[string]$Scenario = "ingest-only")
    Enter-RepoRoot
    try {
        $stream = "events-$Backend"
        $group = "workers-$Backend"
        Set-CommonEnv -RedisStream $stream -RedisGroup $group
        Set-BackendEnv -Backend $Backend
        $env:COLLECTOR_URL = "http://localhost:8080/ingest"
        $env:INGEST_BACKEND = $Backend
        $env:GENERATOR_EPS = "$EPS"
        $env:GENERATOR_BATCH = "$Batch"
        $env:GENERATOR_SEC = "$DurationSec"
        $env:DRAIN_TIMEOUT_SEC = "120"
        $env:DRAIN_POLL_MS = "500"
        $env:DRAIN_STABLE_CHECKS = "3"
        $env:RUN_SCENARIO = $Scenario
        $env:WORKER_WRITE_MODE = $WriteMode
        $env:RUN_TAG = $RunTag
        go run ./cmd/generator
    }
    finally { Leave-RepoRoot }
}

function Run-IngestPostgres { param([int]$EPS,[int]$Batch,[int]$DurationSec = 10,[string]$WriteMode = "batch",[string]$RunTag = "",[string]$Scenario = "ingest-only"); Run-IngestGeneric -Backend "postgres" -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode $WriteMode -RunTag $RunTag -Scenario $Scenario }
function Run-IngestClickHouse { param([int]$EPS,[int]$Batch,[int]$DurationSec = 10,[string]$RunTag = "",[string]$Scenario = "ingest-only"); Run-IngestGeneric -Backend "clickhouse" -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode "batch" -RunTag $RunTag -Scenario $Scenario }
function Run-IngestElasticsearch { param([int]$EPS,[int]$Batch,[int]$DurationSec = 10,[string]$RunTag = "",[string]$Scenario = "ingest-only"); Run-IngestGeneric -Backend "elasticsearch" -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode "batch" -RunTag $RunTag -Scenario $Scenario }
function Run-IngestCassandra { param([int]$EPS,[int]$Batch,[int]$DurationSec = 10,[string]$RunTag = "",[string]$Scenario = "ingest-only"); Run-IngestGeneric -Backend "cassandra" -EPS $EPS -Batch $Batch -DurationSec $DurationSec -WriteMode "batch" -RunTag $RunTag -Scenario $Scenario }

function Start-PostgresStack { param([string]$WriteMode = "batch"); $commonPath = Join-Path $Script:RepoRoot "scripts\common.ps1"; Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-postgres'" | Out-Null; Open-NewPowerShell ". '$commonPath'; Start-WorkerPostgres -WriteMode '$WriteMode'" | Out-Null }
function Start-ClickHouseStack { $commonPath = Join-Path $Script:RepoRoot "scripts\common.ps1"; Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-clickhouse'" | Out-Null; Open-NewPowerShell ". '$commonPath'; Start-WorkerClickHouse" | Out-Null }
function Start-ElasticsearchStack { $commonPath = Join-Path $Script:RepoRoot "scripts\common.ps1"; Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-elasticsearch'" | Out-Null; Open-NewPowerShell ". '$commonPath'; Start-WorkerElasticsearch" | Out-Null }
function Start-CassandraStack { $commonPath = Join-Path $Script:RepoRoot "scripts\common.ps1"; Open-NewPowerShell ". '$commonPath'; Start-Collector -RedisStream 'events-cassandra'" | Out-Null; Open-NewPowerShell ". '$commonPath'; Start-WorkerCassandra" | Out-Null }
