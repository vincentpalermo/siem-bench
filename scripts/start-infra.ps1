$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")

Write-Host "Repository root: $repoRoot"
Write-Host "Starting infrastructure from $repoRoot ..." -ForegroundColor Cyan

Push-Location $repoRoot
try {
    docker compose -f deploy/docker-compose.yml up -d
    Write-Host "Infrastructure started." -ForegroundColor Green
}
finally {
    Pop-Location
}