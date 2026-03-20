cd C:\Users\Vlad\Desktop\siem-bench

$env:COLLECTOR_URL="http://localhost:8080/ingest"
$env:GENERATOR_EPS="2000"
$env:GENERATOR_BATCH="50"
$env:GENERATOR_SEC="10"
$env:GENERATOR_BACKEND="postgres"
$env:POSTGRES_DSN="postgres://siem:siem@127.0.0.1:5432/siem?sslmode=disable"

go run ./cmd/generator