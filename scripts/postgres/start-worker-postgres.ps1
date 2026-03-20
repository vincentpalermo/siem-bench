cd C:\Users\Vlad\Desktop\siem-bench

$env:REDIS_ADDR="127.0.0.1:6379"
$env:REDIS_STREAM="events-postgres"
$env:REDIS_GROUP="workers-postgres"
$env:REDIS_CONSUMER="worker-postgres-1"
$env:POSTGRES_DSN="postgres://siem:siem@127.0.0.1:5432/siem?sslmode=disable"

go run ./cmd/worker-postgres