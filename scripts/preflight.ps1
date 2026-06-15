$ErrorActionPreference = "Continue"

function Test-CommandExists {
    param([string]$Name)
    return $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Test-TcpPort {
    param([string]$HostName, [int]$Port)
    try {
        $client = New-Object System.Net.Sockets.TcpClient
        $async = $client.BeginConnect($HostName, $Port, $null, $null)
        $ok = $async.AsyncWaitHandle.WaitOne(1500, $false)
        if ($ok) { $client.EndConnect($async) }
        $client.Close()
        return $ok
    }
    catch { return $false }
}

function Show-Check {
    param([string]$Name, [bool]$Ok, [string]$Hint = "")
    if ($Ok) {
        Write-Host "[OK]   $Name" -ForegroundColor Green
    }
    else {
        Write-Host "[FAIL] $Name $Hint" -ForegroundColor Red
        $script:Failed = $true
    }
}

$script:Failed = $false

Write-Host "SIEM bench preflight" -ForegroundColor Cyan
Write-Host ""

Show-Check "Docker CLI available" (Test-CommandExists "docker") "Install/start Docker Desktop."
Show-Check "Go CLI available" (Test-CommandExists "go") "Install Go or enable Go toolchain."

if (Test-CommandExists "docker") {
    docker info *> $null
    Show-Check "Docker daemon running" ($LASTEXITCODE -eq 0) "Start Docker Desktop."

    $containers = @(
        "siem-postgres",
        "siem-redis",
        "siem-clickhouse",
        "siem-elasticsearch",
        "siem-cassandra",
        "siem-prometheus",
        "siem-grafana"
    )

    foreach ($name in $containers) {
        $running = docker ps --format "{{.Names}}" | Select-String -SimpleMatch $name
        Show-Check "Container running: $name" ($null -ne $running) "Run .\scripts\start-infra.ps1"
    }
}

$ports = @(
    @{ Name = "PostgreSQL"; Host = "127.0.0.1"; Port = 5432 },
    @{ Name = "Redis"; Host = "127.0.0.1"; Port = 6379 },
    @{ Name = "ClickHouse TCP"; Host = "127.0.0.1"; Port = 9000 },
    @{ Name = "ClickHouse HTTP"; Host = "127.0.0.1"; Port = 8123 },
    @{ Name = "Elasticsearch"; Host = "127.0.0.1"; Port = 9200 },
    @{ Name = "Cassandra"; Host = "127.0.0.1"; Port = 9042 },
    @{ Name = "Prometheus"; Host = "127.0.0.1"; Port = 9090 },
    @{ Name = "Grafana"; Host = "127.0.0.1"; Port = 3000 }
)

foreach ($item in $ports) {
    Show-Check "$($item.Name) port $($item.Port)" (Test-TcpPort -HostName $item.Host -Port $item.Port)
}

$goPorts = @(8080, 2112, 2113, 2114, 2115, 2116)
foreach ($port in $goPorts) {
    $busy = Test-TcpPort -HostName "127.0.0.1" -Port $port
    if ($busy) {
        Write-Host "[WARN] Go service port $port is already busy. Run .\scripts\stop-go-services.ps1 before a clean benchmark." -ForegroundColor Yellow
    }
    else {
        Write-Host "[OK]   Go service port $port is free" -ForegroundColor Green
    }
}

Write-Host ""
if ($script:Failed) {
    Write-Host "Preflight finished with failures." -ForegroundColor Red
    exit 1
}

Write-Host "Preflight passed." -ForegroundColor Green
