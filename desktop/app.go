package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	repoRoot string
	mu       sync.Mutex
	running  bool
}

type AppStatus struct {
	RepoRoot       string `json:"repoRoot"`
	Platform       string `json:"platform"`
	ScriptsReady   bool   `json:"scriptsReady"`
	IsRunning      bool   `json:"isRunning"`
	ResultsSummary string `json:"resultsSummary"`
}

type BenchmarkParams struct {
	Backend             string `json:"backend"`
	Mode                string `json:"mode"`
	EPS                 int    `json:"eps"`
	Batch               int    `json:"batch"`
	DurationSec         int    `json:"durationSec"`
	QueryIntervalSec    int    `json:"queryIntervalSec"`
	QueryWarmupSec      int    `json:"queryWarmupSec"`
	QueryConcurrency    int    `json:"queryConcurrency"`
	WorkerReadCount     int    `json:"workerReadCount"`
	WriteMode           string `json:"writeMode"`
	WorkloadPath        string `json:"workloadPath"`
	ResetStorage        bool   `json:"resetStorage"`
	BuildSummary        bool   `json:"buildSummary"`
}

type CommandResult struct {
	Name     string `json:"name"`
	ExitCode int    `json:"exitCode"`
	Duration string `json:"duration"`
}

type LogLine struct {
	Stream string `json:"stream"`
	Line   string `json:"line"`
	At     string `json:"at"`
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	root, err := findRepoRoot()
	if err != nil {
		a.repoRoot, _ = os.Getwd()
		a.emitLog("stderr", fmt.Sprintf("repo root autodetect failed: %v", err))
		return
	}
	a.repoRoot = root
	a.emitLog("system", "repo root: "+root)
}

func (a *App) GetStatus() AppStatus {
	a.mu.Lock()
	running := a.running
	a.mu.Unlock()

	return AppStatus{
		RepoRoot:       a.repoRoot,
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
		ScriptsReady:   fileExists(filepath.Join(a.repoRoot, "scripts", "run-benchmark.ps1")),
		IsRunning:      running,
		ResultsSummary: a.resultsSummary(),
	}
}

func (a *App) StartInfrastructure() (CommandResult, error) {
	return a.runPowerShellScript("start-infra", filepath.Join("scripts", "start-infra.ps1"), nil)
}

func (a *App) StopGoServices() (CommandResult, error) {
	return a.runPowerShellScript("stop-go-services", filepath.Join("scripts", "stop-go-services.ps1"), nil)
}

func (a *App) RunPreflight() (CommandResult, error) {
	return a.runPowerShellScript("preflight", filepath.Join("scripts", "preflight.ps1"), nil)
}

func (a *App) RunBenchmark(p BenchmarkParams) (CommandResult, error) {
	p = normalizeBenchmarkParams(p)
	args := []string{
		"-Backend", p.Backend,
		"-Mode", p.Mode,
		"-DurationSec", fmt.Sprint(p.DurationSec),
		"-QueryIntervalSec", fmt.Sprint(p.QueryIntervalSec),
		"-QueryWarmupSec", fmt.Sprint(p.QueryWarmupSec),
		"-QueryConcurrency", fmt.Sprint(p.QueryConcurrency),
		"-WorkloadPath", p.WorkloadPath,
		"-WorkerReadCount", fmt.Sprint(p.WorkerReadCount),
		"-WriteMode", p.WriteMode,
	}
	if isIngestMode(p.Mode) {
		args = append(args, "-EPS", fmt.Sprint(p.EPS), "-Batch", fmt.Sprint(p.Batch))
	}
	if p.ResetStorage { args = append(args, "-ResetStorage") }
	if p.BuildSummary { args = append(args, "-BuildSummary") }
	return a.runPowerShellScript("benchmark "+p.Backend+" "+p.Mode, filepath.Join("scripts", "run-benchmark.ps1"), args)
}

func (a *App) OpenResultsFolder() error {
	path := filepath.Join(a.repoRoot, "results")
	if runtime.GOOS == "windows" { return exec.Command("explorer", path).Start() }
	if runtime.GOOS == "darwin" { return exec.Command("open", path).Start() }
	return exec.Command("xdg-open", path).Start()
}

func normalizeBenchmarkParams(p BenchmarkParams) BenchmarkParams {
	if p.Backend == "" { p.Backend = "postgres" }
	if p.Mode == "" { p.Mode = "ingest-only" }
	if p.EPS <= 0 { p.EPS = 100 }
	if p.Batch <= 0 { p.Batch = 20 }
	if p.DurationSec <= 0 { p.DurationSec = 60 }
	if p.QueryIntervalSec <= 0 { p.QueryIntervalSec = 1 }
	if p.QueryWarmupSec <= 0 { p.QueryWarmupSec = 5 }
	if p.QueryConcurrency <= 0 { p.QueryConcurrency = 1 }
	if p.WorkerReadCount <= 0 { p.WorkerReadCount = 100 }
	if p.WriteMode == "" { p.WriteMode = "batch" }
	if p.WorkloadPath == "" { p.WorkloadPath = "scenarios/query-default.json" }
	return p
}

func isIngestMode(mode string) bool {
	switch mode {
	case "ingest", "ingest-only", "mixed", "longrun-ingest", "longrun-mixed": return true
	default: return false
	}
}

func (a *App) runPowerShellScript(name string, scriptRel string, scriptArgs []string) (CommandResult, error) {
	if a.repoRoot == "" { return CommandResult{Name: name, ExitCode: -1}, errors.New("repo root is not initialized") }

	a.mu.Lock()
	if a.running { a.mu.Unlock(); return CommandResult{Name: name, ExitCode: -1}, errors.New("another command is already running") }
	a.running = true
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.running = false; a.mu.Unlock() }()

	start := time.Now()
	scriptPath := filepath.Join(a.repoRoot, scriptRel)
	args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath}
	args = append(args, scriptArgs...)

	a.emitLog("system", "$ powershell "+strings.Join(args, " "))
	cmd := exec.Command("powershell", args...)
	cmd.Dir = a.repoRoot

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil { return CommandResult{Name: name, ExitCode: -1}, err }

	var wg sync.WaitGroup
	wg.Add(2)
	go a.scanPipe(&wg, "stdout", stdout)
	go a.scanPipe(&wg, "stderr", stderr)
	wg.Wait()

	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok { exitCode = exitErr.ExitCode() }
	}

	result := CommandResult{Name: name, ExitCode: exitCode, Duration: time.Since(start).Round(time.Second).String()}
	wailsruntime.EventsEmit(a.ctx, "command-finished", result)
	if exitCode != 0 { return result, fmt.Errorf("%s failed with exit code %d", name, exitCode) }
	return result, nil
}

func (a *App) scanPipe(wg *sync.WaitGroup, stream string, pipe interface{ Read([]byte) (int, error) }) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() { a.emitLog(stream, scanner.Text()) }
}

func (a *App) emitLog(stream, line string) {
	if a.ctx == nil { return }
	wailsruntime.EventsEmit(a.ctx, "command-log", LogLine{Stream: stream, Line: line, At: time.Now().Format("15:04:05")})
}

func (a *App) resultsSummary() string {
	paths := []string{
		filepath.Join(a.repoRoot, "results", "ingest", "summary.csv"),
		filepath.Join(a.repoRoot, "results", "query", "summary.csv"),
		filepath.Join(a.repoRoot, "results", "mixed", "summary-ingest.csv"),
		filepath.Join(a.repoRoot, "results", "longrun-ingest", "summary.csv"),
		filepath.Join(a.repoRoot, "results", "longrun-mixed", "summary-ingest.csv"),
	}
	count := 0
	for _, p := range paths { if fileExists(p) { count++ } }
	return fmt.Sprintf("%d summary files detected", count)
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd(); if err != nil { return "", err }
	for {
		if fileExists(filepath.Join(wd, "scripts", "run-benchmark.ps1")) { return wd, nil }
		parent := filepath.Dir(wd)
		if parent == wd { break }
		wd = parent
	}
	return "", errors.New("scripts/run-benchmark.ps1 not found in parent directories")
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
