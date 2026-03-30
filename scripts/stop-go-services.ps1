$ErrorActionPreference = "Continue"

$ports = @(8080, 2112, 2113, 2114, 2115)

function Get-PidsByPort {
    param(
        [int]$Port
    )

    $pids = @()

    try {
        $connections = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        if ($connections) {
            $pids += $connections | Select-Object -ExpandProperty OwningProcess -Unique
        }
    }
    catch {
    }

    if (-not $pids -or $pids.Count -eq 0) {
        $netstatLines = netstat -ano | Select-String "LISTENING"
        foreach ($line in $netstatLines) {
            $text = $line.ToString().Trim()
            if ($text -match "^\s*TCP\s+\S+:$Port\s+\S+\s+LISTENING\s+(\d+)\s*$") {
                $pids += [int]$matches[1]
            }
        }
    }

    return $pids | Sort-Object -Unique
}

function Stop-PidSafe {
    param(
        [int]$ProcessIdToStop
    )

    if ($ProcessIdToStop -le 0) {
        return
    }

    try {
        taskkill /PID $ProcessIdToStop /F | Out-Null
        Write-Host "Stopped PID $ProcessIdToStop" -ForegroundColor Green
    }
    catch {
        Write-Host "Failed to stop PID ${ProcessIdToStop}: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host "Stopping processes on ports: $($ports -join ', ')" -ForegroundColor Yellow

foreach ($port in $ports) {
    $pids = Get-PidsByPort -Port $port

    if (-not $pids -or $pids.Count -eq 0) {
        Write-Host "No listener found on port $port"
        continue
    }

    Write-Host "Port ${port} is used by PID(s): $($pids -join ', ')"

    foreach ($procId in $pids) {
        Stop-PidSafe -ProcessIdToStop $procId
    }

    Start-Sleep -Milliseconds 500

    $remaining = Get-PidsByPort -Port $port
    if ($remaining -and $remaining.Count -gt 0) {
        Write-Host "Port ${port} is still occupied by PID(s): $($remaining -join ', ')" -ForegroundColor Red
    }
    else {
        Write-Host "Port ${port} is free" -ForegroundColor Green
    }
}

Write-Host "Cleanup complete." -ForegroundColor Cyan