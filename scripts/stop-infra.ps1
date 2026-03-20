$projectRoot = "C:\Users\Vlad\Desktop\siem-bench"

Write-Host "Stopping SIEM bench infrastructure..." -ForegroundColor Cyan

Set-Location $projectRoot

docker compose -f deploy/docker-compose.yml down

if ($LASTEXITCODE -ne 0) {
    Write-Host "Failed to stop Docker infrastructure." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Infrastructure stopped." -ForegroundColor Green