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
	chstorage "siem-bench/internal/storage/clickhouse"
	esstorage "siem-bench/internal/storage/elasticsearch"
	pgstorage "siem-bench/internal/storage/postgres"
)

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func recordQueryMetric(backend, query string, started time.Time, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	metrics.QueryRequestsTotal.WithLabelValues(backend, query, status).Inc()
	metrics.QueryDuration.WithLabelValues(backend, query).Observe(time.Since(started).Seconds())
}

func execQuery(acc *model.QueryAccumulator, backend, name string, fn func() error, collectStats bool) {
	start := time.Now()
	err := fn()
	durationMs := float64(time.Since(start).Microseconds()) / 1000.0

	recordQueryMetric(backend, name, start, err)

	if collectStats {
		acc.Add(durationMs, err != nil)
	}

	if err != nil {
		log.Printf("%s %s error: %v", backend, name, err)
	}
}

func runWorkloadPostgres(workload model.QueryWorkload, backend string, stats map[string]*model.QueryAccumulator, dsn string, intervalSec int, deadline time.Time, collectStats bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	storage, err := pgstorage.New(ctx, dsn)
	cancel()
	if err != nil {
		log.Fatalf("postgres connect failed: %v", err)
	}
	defer storage.Close()

	for time.Now().Before(deadline) {
		for _, q := range workload.Queries {
			if !q.Enabled {
				continue
			}

			switch q.Type {
			case "search_by_host":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.SearchByHost(context.Background(), q.Value, q.Limit)
					return err
				}, collectStats)

			case "search_by_user":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.SearchByUser(context.Background(), q.Value, q.Limit)
					return err
				}, collectStats)

			case "count_by_severity":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.CountBySeverity(context.Background())
					return err
				}, collectStats)

			case "top_hosts":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.TopHosts(context.Background(), q.Limit)
					return err
				}, collectStats)

			default:
				log.Printf("unknown workload query type: %s", q.Type)
			}
		}

		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}

func runWorkloadClickHouse(workload model.QueryWorkload, backend string, stats map[string]*model.QueryAccumulator, dsn string, intervalSec int, deadline time.Time, collectStats bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	storage, err := chstorage.New(ctx, dsn)
	cancel()
	if err != nil {
		log.Fatalf("clickhouse connect failed: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("clickhouse close error: %v", err)
		}
	}()

	for time.Now().Before(deadline) {
		for _, q := range workload.Queries {
			if !q.Enabled {
				continue
			}

			switch q.Type {
			case "search_by_host":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.SearchByHost(context.Background(), q.Value, q.Limit)
					return err
				}, collectStats)

			case "search_by_user":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.SearchByUser(context.Background(), q.Value, q.Limit)
					return err
				}, collectStats)

			case "count_by_severity":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.CountBySeverity(context.Background())
					return err
				}, collectStats)

			case "top_hosts":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.TopHosts(context.Background(), q.Limit)
					return err
				}, collectStats)

			default:
				log.Printf("unknown workload query type: %s", q.Type)
			}
		}

		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}

func runWorkloadElasticsearch(workload model.QueryWorkload, backend string, stats map[string]*model.QueryAccumulator, url string, intervalSec int, deadline time.Time, collectStats bool) {
	storage, err := esstorage.New(url)
	if err != nil {
		log.Fatalf("elasticsearch connect failed: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("elasticsearch close error: %v", err)
		}
	}()

	for time.Now().Before(deadline) {
		for _, q := range workload.Queries {
			if !q.Enabled {
				continue
			}

			switch q.Type {
			case "search_by_host":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.SearchByHost(context.Background(), q.Value, q.Limit)
					return err
				}, collectStats)

			case "search_by_user":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.SearchByUser(context.Background(), q.Value, q.Limit)
					return err
				}, collectStats)

			case "count_by_severity":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.CountBySeverity(context.Background())
					return err
				}, collectStats)

			case "top_hosts":
				acc := stats[q.Name]
				execQuery(acc, backend, q.Name, func() error {
					_, err := storage.TopHosts(context.Background(), q.Limit)
					return err
				}, collectStats)

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

		go func(workerNum int) {
			defer wg.Done()

			switch backend {
			case "postgres":
				runWorkloadPostgres(workload, backend, stats, cfg.PostgresDSN, intervalSec, deadline, collectStats)
			case "clickhouse":
				runWorkloadClickHouse(workload, backend, stats, cfg.ClickHouseDSN, intervalSec, deadline, collectStats)
			case "elasticsearch":
				runWorkloadElasticsearch(workload, backend, stats, cfg.ElasticsearchURL, intervalSec, deadline, collectStats)
			default:
				log.Printf("unsupported QUERY_BACKEND in worker %d: %s", workerNum, backend)
			}
		}(i + 1)
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
	if concurrency <= 0 {
		concurrency = 1
	}

	backend := cfg.QueryBackend
	runScenario := cfg.RunScenario
	if runScenario == "" {
		runScenario = "query-only"
	}

	workload, err := model.LoadQueryWorkload(cfg.QueryWorkloadPath)
	if err != nil {
		log.Fatalf("failed to load query workload: %v", err)
	}

	metrics.RunInfo.WithLabelValues(
		backend,
		runScenario,
		"query",
		"none",
		"none",
	).Set(1)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		log.Printf("query-runner metrics listening on :2114")
		if err := http.ListenAndServe(":2114", mux); err != nil {
			log.Fatalf("query-runner metrics server failed: %v", err)
		}
	}()

	stats := make(map[string]*model.QueryAccumulator)
	resultWorkload := make([]model.QueryWorkloadItem, 0, len(workload.Queries))

	for _, q := range workload.Queries {
		resultWorkload = append(resultWorkload, q)
		if q.Enabled {
			stats[q.Name] = &model.QueryAccumulator{Name: q.Name}
		}
	}

	startedAt := time.Now().UTC()
	runID := startedAt.Format("20060102-150405")

	log.Printf(
		"query-runner started: backend=%s duration=%ds interval=%ds warmup=%ds concurrency=%d scenario=%s workload=%s",
		backend, durationSec, intervalSec, warmupSec, concurrency, runScenario, workload.Name,
	)

	if warmupSec > 0 {
		warmupDeadline := time.Now().Add(time.Duration(warmupSec) * time.Second)
		log.Printf("starting warm-up phase for %ds", warmupSec)

		runConcurrent(workload, backend, stats, cfg, intervalSec, warmupDeadline, false, concurrency)

		log.Printf("warm-up phase finished")
	}

	measureDeadline := time.Now().Add(time.Duration(durationSec) * time.Second)
	runConcurrent(workload, backend, stats, cfg, intervalSec, measureDeadline, true, concurrency)

	finishedAt := time.Now().UTC()

	queryStats := make([]model.QueryStat, 0, len(stats))
	totalQueries := 0
	failedQueries := 0

	for _, acc := range stats {
		stat := acc.ToStat()
		queryStats = append(queryStats, stat)
		totalQueries += stat.Count
		failedQueries += stat.Failed
	}

	result := model.QueryRunResult{
		RunID:         runID,
		Backend:       backend,
		TotalQueries:  totalQueries,
		FailedQueries: failedQueries,
		Notes:         cfg.RunTag,
		ConfigSnapshot: model.QueryConfigSnapshot{
			Backend:      backend,
			DurationSec:  durationSec,
			IntervalSec:  intervalSec,
			WarmupSec:    warmupSec,
			Concurrency:  concurrency,
			RunScenario:  runScenario,
			WorkloadName: workload.Name,
			WorkloadPath: cfg.QueryWorkloadPath,
		},
		Workload:   resultWorkload,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Queries:    queryStats,
	}

	resultPath := ""
	switch runScenario {
	case "mixed":
		resultPath = fmt.Sprintf("results/mixed/query-%s-%s.json", backend, runID)
	default:
		resultPath = fmt.Sprintf("results/query/query-%s-%s.json", backend, runID)
	}

	if err := model.SaveQueryRunResult(resultPath, result); err != nil {
		log.Fatalf("failed to save query result file: %v", err)
	}

	log.Printf(
		"query-runner finished: backend=%s total_queries=%d failed_queries=%d workload=%s concurrency=%d result=%s",
		backend, totalQueries, failedQueries, workload.Name, concurrency, resultPath,
	)
}