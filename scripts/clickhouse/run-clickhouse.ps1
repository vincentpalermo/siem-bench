$projectRoot = "C:\Users\Vlad\Desktop\siem-bench"

function Ask-Int($prompt, $defaultValue) {
    while ($true) {
        $value = Read-Host "$prompt [$defaultValue]"
        if ([string]::IsNullOrWhiteSpace($value)) {
            return [int]$defaultValue
        }

        $parsed = 0
        if ([int]::TryParse($value, [ref]$parsed)) {
            return $parsed
        }

        Write-Host "Please enter a valid integer." -ForegroundColor Yellow
    }
}

function Ask-YesNo($prompt, $defaultValue) {
    while ($true) {
        $value = Read-Host "$prompt [$defaultValue]"
        if ([string]::IsNullOrWhiteSpace($value)) {
            $value = $defaultValue
        }

        $normalized = $value.ToLower()
        if ($normalized -in @("y", "yes")) {
            return $true
        }
        if ($normalized -in @("n", "no")) {
            return $false
        }

        Write-Host "Please answer y/yes or n/no." -ForegroundColor Yellow
    }
}

$eps = Ask-Int "Enter EPS" 500
$batch = Ask-Int "Enter batch size" 10
$durationSec = Ask-Int "Enter test duration (sec)" 10
$withQuery = Ask-YesNo "Run query-runner? (y/n)" "y"
$resetTable = Ask-YesNo "Reset ClickHouse table before test? (y/n)" "n"

Write-Host ""
Write-Host "ClickHouse scenario parameters:" -ForegroundColor Cyan
Write-Host "EPS=$eps"
Write-Host "Batch=$batch"
Write-Host "DurationSec=$durationSec"
Write-Host "WithQuery=$withQuery"
Write-Host "ResetTable=$resetTable"
Write-Host ""

if ($resetTable) {
    Write-Host "Resetting ClickHouse table..." -ForegroundColor Cyan
    docker exec -i siem-clickhouse clickhouse-client --user siem --password siem --query "TRUNCATE TABLE siem.events"
}

Write-Host "Starting collector..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", @"
cd $projectRoot
`$env:REDIS_ADDR='127.0.0.1:6379'
`$env:REDIS_STREAM='events-clickhouse'
`$env:HTTP_ADDR=':8080'
go run ./cmd/collector
"@

Write-Host "Starting worker-clickhouse..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", @"
cd $projectRoot
`$env:REDIS_ADDR='127.0.0.1:6379'
`$env:REDIS_STREAM='events-clickhouse'
`$env:REDIS_GROUP='workers-clickhouse'
`$env:REDIS_CONSUMER='worker-clickhouse-1'
`$env:CLICKHOUSE_DSN='clickhouse://siem:siem@127.0.0.1:9000/siem'
go run ./cmd/worker-clickhouse
"@

if ($withQuery) {
    Write-Host "Starting query-runner..." -ForegroundColor Cyan
    Start-Process powershell -ArgumentList "-NoExit", "-Command", @"
cd $projectRoot
`$env:GENERATOR_BACKEND='clickhouse'
`$env:CLICKHOUSE_DSN='clickhouse://siem:siem@127.0.0.1:9000/siem'
go run ./cmd/query-runner
"@
}

Write-Host "Waiting for services to initialize..." -ForegroundColor Cyan
Start-Sleep -Seconds 5

Write-Host "Starting generator..." -ForegroundColor Cyan
cd $projectRoot
$env:COLLECTOR_URL = "http://localhost:8080/ingest"
$env:GENERATOR_EPS = "$eps"
$env:GENERATOR_BATCH = "$batch"
$env:GENERATOR_SEC = "$durationSec"
$env:GENERATOR_BACKEND = "clickhouse"
$env:CLICKHOUSE_DSN = "clickhouse://siem:siem@127.0.0.1:9000/siem"

go run ./cmd/generator