cd C:\Users\Vlad\Desktop\siem-bench

$env:REDIS_ADDR="127.0.0.1:6379"
$env:REDIS_STREAM="events-clickhouse"
$env:HTTP_ADDR=":8080"

go run ./cmd/collector