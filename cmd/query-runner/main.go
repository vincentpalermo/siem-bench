package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
	if err != nil || parsed <= 0 {
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

func execQuery(acc *model.QueryAccumulator, backend, name string, fn func() error) {
	start := time.Now()
	err := fn()
	durationMs := float64(time.Since(start).Microseconds()) / 1000.0

	recordQueryMetric(backend, name, start, err)
	acc.Add(durationMs, err != nil)

	if err != nil {
		log.Printf("%s %s error: %v", backend, name, err)
	}
}

func saveQueryResult(path string, result model.QueryRunResult) error {
	if err := os.MkdirAll("results", 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	return enc.Encode(result)
}

func runPostgresQueries(deadline time.Time, backend string, stats map[string]*model.QueryAccumulator, dsn string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	storage, err := pgstorage.New(ctx, dsn)
	cancel()
	if err != nil {
		log.Fatalf("postgres connect failed: %v", err)
	}
	defer storage.Close()

	intervalSec := getEnvInt("QUERY_RUNNER_INTERVAL_SEC", 2)

	for time.Now().Before(deadline) {
		execQuery(stats["search_by_host"], backend, "search_by_host", func() error {
			_, err := storage.SearchByHost(context.Background(), "host-1", 100)
			return err
		})

		execQuery(stats["search_by_user"], backend, "search_by_user", func() error {
			_, err := storage.SearchByUser(context.Background(), "admin", 100)
			return err
		})

		execQuery(stats["count_by_severity"], backend, "count_by_severity", func() error {
			_, err := storage.CountBySeverity(context.Background())
			return err
		})

		execQuery(stats["top_hosts"], backend, "top_hosts", func() error {
			_, err := storage.TopHosts(context.Background(), 10)
			return err
		})

		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}

func runClickHouseQueries(deadline time.Time, backend string, stats map[string]*model.QueryAccumulator, dsn string) {
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

	intervalSec := getEnvInt("QUERY_RUNNER_INTERVAL_SEC", 2)

	for time.Now().Before(deadline) {
		execQuery(stats["search_by_host"], backend, "search_by_host", func() error {
			_, err := storage.SearchByHost(context.Background(), "host-1", 100)
			return err
		})

		execQuery(stats["search_by_user"], backend, "search_by_user", func() error {
			_, err := storage.SearchByUser(context.Background(), "admin", 100)
			return err
		})

		execQuery(stats["count_by_severity"], backend, "count_by_severity", func() error {
			_, err := storage.CountBySeverity(context.Background())
			return err
		})

		execQuery(stats["top_hosts"], backend, "top_hosts", func() error {
			_, err := storage.TopHosts(context.Background(), 10)
			return err
		})

		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}

func runElasticsearchQueries(deadline time.Time, backend string, stats map[string]*model.QueryAccumulator, url string) {
	storage, err := esstorage.New(url)
	if err != nil {
		log.Fatalf("elasticsearch connect failed: %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			log.Printf("elasticsearch close error: %v", err)
		}
	}()

	intervalSec := getEnvInt("QUERY_RUNNER_INTERVAL_SEC", 2)

	for time.Now().Before(deadline) {
		execQuery(stats["search_by_host"], backend, "search_by_host", func() error {
			_, err := storage.SearchByHost(context.Background(), "host-1", 100)
			return err
		})

		execQuery(stats["search_by_user"], backend, "search_by_user", func() error {
			_, err := storage.SearchByUser(context.Background(), "admin", 100)
			return err
		})

		execQuery(stats["count_by_severity"], backend, "count_by_severity", func() error {
			_, err := storage.CountBySeverity(context.Background())
			return err
		})

		execQuery(stats["top_hosts"], backend, "top_hosts", func() error {
			_, err := storage.TopHosts(context.Background(), 10)
			return err
		})

		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}

func main() {
	cfg := config.Load()
	metrics.MustRegister()

	durationSec := getEnvInt("QUERY_RUNNER_DURATION_SEC", 10)
	backend := cfg.GeneratorBackend

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())

		log.Printf("query-runner metrics listening on :2114")
		if err := http.ListenAndServe(":2114", mux); err != nil {
			log.Fatalf("query-runner metrics server failed: %v", err)
		}
	}()

	stats := map[string]*model.QueryAccumulator{
		"search_by_host":    {Name: "search_by_host"},
		"search_by_user":    {Name: "search_by_user"},
		"count_by_severity": {Name: "count_by_severity"},
		"top_hosts":         {Name: "top_hosts"},
	}

	startedAt := time.Now().UTC()
	runID := startedAt.Format("20060102-150405")
	deadline := time.Now().Add(time.Duration(durationSec) * time.Second)

	log.Printf("query-runner started: backend=%s duration=%ds interval=%ds",
		backend,
		durationSec,
		getEnvInt("QUERY_RUNNER_INTERVAL_SEC", 2),
	)

	switch backend {
	case "postgres":
		runPostgresQueries(deadline, backend, stats, cfg.PostgresDSN)
	case "clickhouse":
		runClickHouseQueries(deadline, backend, stats, cfg.ClickHouseDSN)
	case "elasticsearch":
		runElasticsearchQueries(deadline, backend, stats, cfg.ElasticsearchURL)
	default:
		log.Fatalf("unsupported GENERATOR_BACKEND: %s", backend)
	}

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
		DurationSec:   durationSec,
		TotalQueries:  totalQueries,
		FailedQueries: failedQueries,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		Queries:       queryStats,
	}

	resultPath := fmt.Sprintf("results/query-%s-%s.json", backend, runID)
	if err := saveQueryResult(resultPath, result); err != nil {
		log.Fatalf("failed to save query result file: %v", err)
	}

	log.Printf("query-runner finished: backend=%s total_queries=%d failed_queries=%d result=%s",
		backend, totalQueries, failedQueries, resultPath)
}
