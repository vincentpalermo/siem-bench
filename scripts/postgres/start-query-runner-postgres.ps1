cd C:\Users\Vlad\Desktop\siem-bench

$env:GENERATOR_BACKEND="postgres"
$env:POSTGRES_DSN="postgres://siem:siem@127.0.0.1:5432/siem?sslmode=disable"

go run ./cmd/query-runner