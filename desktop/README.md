# SIEM Bench Control Center

Desktop control panel for the SIEM-like benchmark stand.

## Features

- Start Docker infrastructure from `scripts/start-infra.ps1`.
- Stop running Go services from `scripts/stop-go-services.ps1`.
- Run preflight checks from `scripts/preflight.ps1`.
- Launch benchmark scenarios from `scripts/run-benchmark.ps1`.
- Select backend, mode, EPS, batch size, duration, query concurrency and other parameters.
- Stream PowerShell logs directly into the desktop UI.

## Requirements

Install Wails CLI and verify the environment:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

Wails requires Go, Node/NPM and WebView2 on Windows.

## Development

From the repository root:

```powershell
cd desktop
npm install --prefix frontend
wails dev
```

## Build

```powershell
cd desktop
wails build
```

The executable will be generated under `desktop/build/bin`.

## Notes

The app auto-detects the repository root by searching parent folders for `scripts/run-benchmark.ps1`. Run it from the `desktop` directory or from anywhere inside the repository.
