$env:COLLECTOR_URL="http://localhost:8080/ingest"
$env:GENERATOR_EPS="500"
$env:GENERATOR_BATCH="10"
$env:GENERATOR_SEC="10"
$env:POSTGRES_DSN="postgres://siem:siem@localhost:5432/siem?sslmode=disable"

go run ./cmd/generator