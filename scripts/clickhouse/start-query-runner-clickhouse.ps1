cd C:\Users\Vlad\Desktop\siem-bench

$env:GENERATOR_BACKEND="clickhouse"
$env:CLICKHOUSE_DSN="clickhouse://siem:siem@127.0.0.1:9000/siem"

go run ./cmd/query-runner