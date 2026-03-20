$projectRoot = "C:\Users\Vlad\Desktop\siem-bench"

Write-Host "Starting SIEM bench infrastructure..." -ForegroundColor Cyan

Set-Location $projectRoot

docker compose -f deploy/docker-compose.yml up -d

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to start Docker infrastructure." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Docker containers started." -ForegroundColor Green
Write-Host "Checking running containers..." -ForegroundColor Cyan

docker ps

Write-Host ""
Write-Host "Available services:" -ForegroundColor Cyan
Write-Host "Prometheus: http://localhost:9090"
Write-Host "Grafana:    http://localhost:3000"
Write-Host "PostgreSQL: localhost:5432"
Write-Host "Redis:      localhost:6379"
Write-Host "ClickHouse: localhost:8123 / 9000"
Write-Host ""
Write-Host "Infrastructure is ready." -ForegroundColor Green