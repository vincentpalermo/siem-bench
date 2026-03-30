$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")

Write-Host "Repository root: $repoRoot"
Write-Host "Stopping infrastructure from $repoRoot ..." -ForegroundColor Yellow

Push-Location $repoRoot
try {
    docker compose -f deploy/docker-compose.yml down
    Write-Host "Infrastructure stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}