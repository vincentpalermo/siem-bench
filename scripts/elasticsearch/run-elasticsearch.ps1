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
$resetIndex = Ask-YesNo "Reset Elasticsearch index before test? (y/n)" "n"

Write-Host ""
Write-Host "Elasticsearch scenario parameters:" -ForegroundColor Cyan
Write-Host "EPS=$eps"
Write-Host "Batch=$batch"
Write-Host "DurationSec=$durationSec"
Write-Host "WithQuery=$withQuery"
Write-Host "ResetIndex=$resetIndex"
Write-Host ""

if ($resetIndex) {
    Write-Host "Resetting Elasticsearch index..." -ForegroundColor Cyan
    try {
        Invoke-RestMethod -Method DELETE -Uri "http://127.0.0.1:9200/siem-events" | Out-Null
    } catch {
        Write-Host "Index delete skipped or failed; continuing..." -ForegroundColor Yellow
    }

    $body = @'
{
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "timestamp":   { "type": "date" },
      "source_type": { "type": "keyword" },
      "host":        { "type": "keyword" },
      "user_name":   { "type": "keyword" },
      "src_ip":      { "type": "ip" },
      "dst_ip":      { "type": "ip" },
      "event_code":  { "type": "keyword" },
      "severity":    { "type": "integer" },
      "message":     { "type": "text" },
      "raw":         { "type": "text" }
    }
  }
}
'@

    Invoke-RestMethod -Method PUT -Uri "http://127.0.0.1:9200/siem-events" -ContentType "application/json" -Body $body | Out-Null
}

Write-Host "Starting collector..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", @"
cd $projectRoot
`$env:REDIS_ADDR='127.0.0.1:6379'
`$env:REDIS_STREAM='events-elasticsearch'
`$env:HTTP_ADDR=':8080'
go run ./cmd/collector
"@

Write-Host "Starting worker-elasticsearch..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", @"
cd $projectRoot
`$env:REDIS_ADDR='127.0.0.1:6379'
`$env:REDIS_STREAM='events-elasticsearch'
`$env:REDIS_GROUP='workers-elasticsearch'
`$env:REDIS_CONSUMER='worker-elasticsearch-1'
`$env:ELASTICSEARCH_URL='http://127.0.0.1:9200'
go run ./cmd/worker-elasticsearch
"@

if ($withQuery) {
    Write-Host "Starting query-runner..." -ForegroundColor Cyan
    Start-Process powershell -ArgumentList "-NoExit", "-Command", @"
cd $projectRoot
`$env:GENERATOR_BACKEND='elasticsearch'
`$env:ELASTICSEARCH_URL='http://127.0.0.1:9200'
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
$env:GENERATOR_BACKEND = "elasticsearch"
$env:ELASTICSEARCH_URL = "http://127.0.0.1:9200"

go run ./cmd/generator