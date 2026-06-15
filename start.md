powershell -ExecutionPolicy Bypass -File .\scripts\run-benchmark.ps1 -Backend postgres -Mode ingest -EPS 300 -Batch 20 -DurationSec 20 -WorkerReadCount 200 -WriteMode batch -RunTag demo-postgres-300eps -ResetStorage -StartCollector -StartWorker -BuildSummary

powershell -ExecutionPolicy Bypass -File .\scripts\run-benchmark.ps1 -Backend clickhouse -Mode ingest -EPS 300 -Batch 20 -DurationSec 20 -WorkerReadCount 200 -WriteMode batch -RunTag demo-clickhouse-300eps -ResetStorage -StartCollector -StartWorker -BuildSummary

powershell -ExecutionPolicy Bypass -File .\scripts\run-benchmark.ps1 -Backend elasticsearch -Mode ingest -EPS 300 -Batch 20 -DurationSec 20 -WorkerReadCount 200 -WriteMode batch -RunTag demo-elasticsearch-300eps -ResetStorage -StartCollector -StartWorker -BuildSummary
