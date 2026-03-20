Write-Host "Stopping SIEM bench Go services..." -ForegroundColor Cyan

$ports = @(8080, 2112, 2113, 2114)

foreach ($port in $ports) {
    try {
        $connections = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
        if ($connections) {
            $pids = $connections | Select-Object -ExpandProperty OwningProcess -Unique
            foreach ($pid in $pids) {
                try {
                    $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
                    if ($proc) {
                        Write-Host "Stopping process on port $port: PID=$pid Name=$($proc.ProcessName)" -ForegroundColor Yellow
                        Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
                    }
                } catch {
                    Write-Host "Failed to stop PID $pid on port $port" -ForegroundColor Red
                }
            }
        } else {
            Write-Host "No process found on port $port" -ForegroundColor DarkGray
        }
    } catch {
        Write-Host "Could not inspect port $port" -ForegroundColor Red
    }
}

Start-Sleep -Seconds 1

# Optional extra cleanup: terminate leftover 'go' processes started from terminals
$goProcesses = Get-Process go -ErrorAction SilentlyContinue
if ($goProcesses) {
    foreach ($proc in $goProcesses) {
        try {
            Write-Host "Stopping leftover go process: PID=$($proc.Id)" -ForegroundColor Yellow
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        } catch {
            Write-Host "Failed to stop go process PID=$($proc.Id)" -ForegroundColor Red
        }
    }
} else {
    Write-Host "No leftover go processes found" -ForegroundColor DarkGray
}

Write-Host "Cleanup finished." -ForegroundColor Green