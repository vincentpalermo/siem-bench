$env:COLLECTOR_URL="http://localhost:8080/ingest"
$env:GENERATOR_EPS="1000"
$env:GENERATOR_BATCH="20"
$env:GENERATOR_SEC="10"
$env:POSTGRES_DSN="postgres://siem:siem@localhost:5432/siem?sslmode=disable"

go run ./cmd/generator