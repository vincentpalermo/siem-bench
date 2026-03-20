cd C:\Users\Vlad\Desktop\siem-bench

$env:REDIS_ADDR="127.0.0.1:6379"
$env:REDIS_STREAM="events-clickhouse"
$env:REDIS_GROUP="workers-clickhouse"
$env:REDIS_CONSUMER="worker-clickhouse-1"
$env:CLICKHOUSE_DSN="clickhouse://siem:siem@127.0.0.1:9000/siem"

go run ./cmd/worker-clickhouse