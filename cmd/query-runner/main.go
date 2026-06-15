package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"siem-bench/internal/config"
	"siem-bench/internal/metrics"
	"siem-bench/internal/model"
	"siem-bench/internal/reporting"
	cassandrastorage "siem-bench/internal/storage/cassandra"
	chstorage "siem-bench/internal/storage/clickhouse"
	esstorage "siem-bench/internal/storage/elasticsearch"
	pgstorage "siem-bench/internal/storage/postgres"
)

type queryStorage interface {
	SearchByHost(context.Context, string, int) ([]model.EventQueryResult, error)
	SearchByUser(context.Context, string, int) ([]model.EventQueryResult, error)
	CountBySeverity(context.Context) ([]model.SeverityCount, error)
	TopHosts(context.Context, int) ([]model.HostCount, error)
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" { return fallback }
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed < 0 { return fallback }
	return parsed
}

func queryResultPath(scenario, backend, runID string) string {
	switch scenario {
	case "mixed":
		return fmt.Sprintf("results/mixed/query-%s-%s.json", backend, runID)
	case "longrun-mixed":
		return fmt.Sprintf("results/longrun-mixed/query-%s-%s.json", backend, runID)
	default:
		return fmt.Sprintf("results/query/query-%s-%s.json", backend, runID)
	}
}

func recordQueryMetric(backend, query string, started time.Time, err error) {
	status := "ok"
	if err != nil { status = "error" }
	metrics.QueryRequestsTotal.WithLabelValues(backend, query, status).Inc()
	metrics.QueryDuration.WithLabelValues(backend, query).Observe(time.Since(started).Seconds())
}

func execQuery(acc *model.QueryAccumulator, backend, name string, fn func() error, collectStats bool) {
	started := time.Now()
	err := fn()
	durationMs := float64(time.Since(started).Microseconds()) / 1000.0
	recordQueryMetric(backend, name, started, err)
	if collectStats { acc.Add(durationMs, err != nil) }
	if err != nil { log.Printf("%s %s error: %v", backend, name, err) }
}

func openStorage(ctx context.Context, cfg config.Config, backend string) (queryStorage, func()) {
	switch backend {
	case "postgres":
		s, err := pgstorage.New(ctx, cfg.PostgresDSN); if err != nil { log.Fatalf("postgres connect failed: %v", err) }
		return s, s.Close
	case "clickhouse":
		s, err := chstorage.New(ctx, cfg.ClickHouseDSN); if err != nil { log.Fatalf("clickhouse connect failed: %v", err) }
		return s, func(){ if err := s.Close(); err != nil { log.Printf("clickhouse close error: %v", err) } }
	case "elasticsearch":
		s, err := esstorage.New(cfg.ElasticsearchURL); if err != nil { log.Fatalf("elasticsearch connect failed: %v", err) }
		return s, func(){ if err := s.Close(); err != nil { log.Printf("elasticsearch close error: %v", err) } }
	case "cassandra":
		s, err := cassandrastorage.New("localhost"); if err != nil { log.Fatalf("cassandra connect failed: %v", err) }
		return s, s.Close
	default:
		log.Fatalf("unsupported QUERY_BACKEND: %s", backend)
	}
	return nil, func(){}
}

func runWorkload(workload model.QueryWorkload, backend string, stats map[string]*model.QueryAccumulator, cfg config.Config, intervalSec int, deadline time.Time, collectStats bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	storage, closeStorage := openStorage(ctx, cfg, backend)
	cancel()
	defer closeStorage()
	for time.Now().Before(deadline) {
		for _, q := range workload.Queries {
			if !q.Enabled { continue }
			acc := stats[q.Name]
			switch q.Type {
			case "search_by_host":
				execQuery(acc, backend, q.Name, func() error { _, err := storage.SearchByHost(context.Background(), q.Value, q.Limit); return err }, collectStats)
			case "search_by_user":
				execQuery(acc, backend, q.Name, func() error { _, err := storage.SearchByUser(context.Background(), q.Value, q.Limit); return err }, collectStats)
			case "count_by_severity":
				execQuery(acc, backend, q.Name, func() error { _, err := storage.CountBySeverity(context.Background()); return err }, collectStats)
			case "top_hosts":
				execQuery(acc, backend, q.Name, func() error { _, err := storage.TopHosts(context.Background(), q.Limit); return err }, collectStats)
			default:
				log.Printf("unknown workload query type: %s", q.Type)
			}
		}
		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}

func runConcurrent(workload model.QueryWorkload, backend string, stats map[string]*model.QueryAccumulator, cfg config.Config, intervalSec int, deadline time.Time, collectStats bool, concurrency int) {
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(){ defer wg.Done(); runWorkload(workload, backend, stats, cfg, intervalSec, deadline, collectStats) }()
	}
	wg.Wait()
}

func main() {
	cfg := config.Load()
	metrics.MustRegister()
	durationSec := getEnvInt("QUERY_RUNNER_DURATION_SEC", 10)
	intervalSec := getEnvInt("QUERY_RUNNER_INTERVAL_SEC", 2)
	warmupSec := getEnvInt("QUERY_RUNNER_WARMUP_SEC", 3)
	concurrency := getEnvInt("QUERY_RUNNER_CONCURRENCY", 1)
	if concurrency <= 0 { concurrency = 1 }
	backend := cfg.QueryBackend
	runScenario := cfg.RunScenario
	if runScenario == "" { runScenario = "query-only" }
	workload, err := model.LoadQueryWorkload(cfg.QueryWorkloadPath)
	if err != nil { log.Fatalf("failed to load query workload: %v", err) }
	metrics.RunInfo.WithLabelValues(backend, runScenario, "query", "none", "none").Set(1)
	go func(){ mux := http.NewServeMux(); mux.Handle("/metrics", promhttp.Handler()); log.Printf("query-runner metrics listening on :2114"); if err := http.ListenAndServe(":2114", mux); err != nil { log.Fatalf("query-runner metrics server failed: %v", err) } }()
	stats := make(map[string]*model.QueryAccumulator)
	resultWorkload := make([]model.QueryWorkloadItem, 0, len(workload.Queries))
	for _, q := range workload.Queries { resultWorkload = append(resultWorkload, q); if q.Enabled { stats[q.Name] = &model.QueryAccumulator{Name: q.Name} } }
	startedAt := time.Now().UTC()
	runID := startedAt.Format("20060102-150405")
	log.Printf("query-runner started: backend=%s duration=%ds interval=%ds warmup=%ds concurrency=%d scenario=%s workload=%s", backend, durationSec, intervalSec, warmupSec, concurrency, runScenario, workload.Name)
	if warmupSec > 0 {
		log.Printf("starting warm-up phase for %ds", warmupSec)
		runConcurrent(workload, backend, stats, cfg, intervalSec, time.Now().Add(time.Duration(warmupSec)*time.Second), false, concurrency)
		log.Printf("warm-up phase finished")
	}
	runConcurrent(workload, backend, stats, cfg, intervalSec, time.Now().Add(time.Duration(durationSec)*time.Second), true, concurrency)
	finishedAt := time.Now().UTC()
	sysSnap, err := reporting.FetchSystemMetricsForRun(backend, startedAt, finishedAt); if err != nil { log.Printf("failed to fetch system metrics: %v", err) }
	queryStats := make([]model.QueryStat, 0, len(stats))
	totalQueries, failedQueries := 0, 0
	for _, acc := range stats { stat := acc.ToStat(); queryStats = append(queryStats, stat); totalQueries += stat.Count; failedQueries += stat.Failed }
	result := model.QueryRunResult{RunID: runID, Backend: backend, TotalQueries: totalQueries, FailedQueries: failedQueries, Notes: cfg.RunTag, SystemCPUAvgPercent: sysSnap.CPUAvgPercent, SystemCPUMaxPercent: sysSnap.CPUMaxPercent, SystemMemoryAvgMB: sysSnap.MemoryAvgMB, SystemMemoryMaxMB: sysSnap.MemoryMaxMB, SystemDiskReadMB: sysSnap.DiskReadMB, SystemDiskWriteMB: sysSnap.DiskWriteMB, SystemNetRxMB: sysSnap.NetRxMB, SystemNetTxMB: sysSnap.NetTxMB, ConfigSnapshot: model.QueryConfigSnapshot{Backend: backend, DurationSec: durationSec, IntervalSec: intervalSec, WarmupSec: warmupSec, Concurrency: concurrency, RunScenario: runScenario, WorkloadName: workload.Name, WorkloadPath: cfg.QueryWorkloadPath}, Workload: resultWorkload, StartedAt: startedAt, FinishedAt: finishedAt, Queries: queryStats}
	path := queryResultPath(runScenario, backend, runID)
	if err := model.SaveQueryRunResult(path, result); err != nil { log.Fatalf("failed to save query result file: %v", err) }
	log.Printf("query-runner finished: backend=%s total_queries=%d failed_queries=%d result=%s", backend, totalQueries, failedQueries, path)
}
