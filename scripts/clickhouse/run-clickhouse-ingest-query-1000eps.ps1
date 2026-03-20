cd C:\Users\Vlad\Desktop\siem-bench

$env:COLLECTOR_URL="http://localhost:8080/ingest"
$env:GENERATOR_EPS="1000"
$env:GENERATOR_BATCH="20"
$env:GENERATOR_SEC="10"
$env:GENERATOR_BACKEND="clickhouse"
$env:CLICKHOUSE_DSN="clickhouse://siem:siem@127.0.0.1:9000/siem"

go run ./cmd/generator